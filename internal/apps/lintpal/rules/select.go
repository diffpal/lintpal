package rules

import (
	"context"
	"encoding/hex"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

const maxSelections = 100_000
const maxSelectedRuleBytes = 16 << 20

var ErrInvalidSelector = errors.New("invalid rule path selector")

// Selection binds one validated rule to one immutable Git work item.
type Selection struct {
	GroupID    string
	Item       git.WorkItem
	Rule       Rule
	QuestionID string
}

// Select applies declarative path/side selectors in canonical item/rule order.
func Select(ctx context.Context, pack Pack, groups []contextplan.Group) ([]Selection, error) {
	return SelectWithPaths(ctx, pack, groups, nil, nil)
}

// SelectWithPaths applies run-level source-path filters before rule selection.
// A matching exclude takes precedence over every include.
func SelectWithPaths(ctx context.Context, pack Pack, groups []contextplan.Group, includes, excludes []string) ([]Selection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(pack.rules) == 0 {
		return nil, ErrInvalidPack
	}
	if err := ValidateSelectors(includes, excludes); err != nil {
		return nil, err
	}
	orderedGroups := append([]contextplan.Group(nil), groups...)
	for i, group := range orderedGroups {
		if group.ID == "" || len(group.Items) == 0 {
			return nil, ErrInvalidRule
		}
		orderedGroups[i].Items = append([]git.WorkItem(nil), group.Items...)
		sort.Slice(orderedGroups[i].Items, func(a, b int) bool {
			return itemLess(orderedGroups[i].Items[a], orderedGroups[i].Items[b])
		})
	}
	sort.Slice(orderedGroups, func(i, j int) bool {
		return itemLess(orderedGroups[i].Items[0], orderedGroups[j].Items[0])
	})
	rules := pack.Rules()
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	seenItems := make(map[string]bool)
	seenGroups := make(map[string]bool)
	selections := make([]Selection, 0)
	selectedBytes := 0
	for _, group := range orderedGroups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seenGroups[group.ID] {
			return nil, ErrInvalidRule
		}
		seenGroups[group.ID] = true
		items := append([]git.WorkItem(nil), group.Items...)
		sort.Slice(items, func(i, j int) bool { return itemLess(items[i], items[j]) })
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !validSelectionItem(item) || seenItems[item.ID] {
				return nil, ErrInvalidRule
			}
			seenItems[item.ID] = true
			if !selectedPath(item.Path, includes, excludes) {
				continue
			}
			for _, rule := range rules {
				if !applies(rule, item) {
					continue
				}
				if len(selections) >= maxSelections {
					return nil, ErrPackLimit
				}
				weight := ruleBytes(rule)
				if weight > maxSelectedRuleBytes-selectedBytes {
					return nil, ErrPackLimit
				}
				selectedBytes += weight
				selections = append(selections, Selection{GroupID: group.ID, Item: item,
					Rule: cloneRule(rule), QuestionID: item.ID + "/" + rule.ID})
			}
		}
	}
	return selections, nil
}

// ValidateSelectors checks the same safe glob subset used by rule selection.
func ValidateSelectors(includes, excludes []string) error {
	if len(includes) > 32 || len(excludes) > 32 {
		return ErrInvalidSelector
	}
	for _, pattern := range append(append([]string(nil), includes...), excludes...) {
		if pattern == "" || len(pattern) > maxPattern || strings.Contains(pattern, "\\") ||
			strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "..") {
			return ErrInvalidSelector
		}
		if _, err := path.Match(pattern, "example.go"); err != nil {
			return ErrInvalidSelector
		}
	}
	return nil
}

func selectedPath(target string, includes, excludes []string) bool {
	if len(includes) > 0 && !matchesAny(target, includes) {
		return false
	}
	return !matchesAny(target, excludes)
}

func matchesAny(target string, patterns []string) bool {
	for _, pattern := range patterns {
		candidate := target
		if !strings.Contains(pattern, "/") {
			candidate = path.Base(target)
		}
		if match, _ := path.Match(pattern, candidate); match {
			return true
		}
	}
	return false
}

func itemLess(a, b git.WorkItem) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Side != b.Side {
		return a.Side < b.Side
	}
	if a.Hunk != b.Hunk {
		return a.Hunk < b.Hunk
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	if a.EndLine != b.EndLine {
		return a.EndLine < b.EndLine
	}
	return a.ID < b.ID
}

func validSelectionItem(item git.WorkItem) bool {
	if len(item.ID) != 64 || item.Path == "" || item.StartLine < 1 || item.EndLine < item.StartLine || item.Hunk < 1 {
		return false
	}
	if _, err := hex.DecodeString(item.ID); err != nil {
		return false
	}
	if item.Side == git.Left {
		return item.Path == item.OldPath
	}
	if item.Side == git.Right {
		return item.Path == item.NewPath
	}
	return false
}

func applies(rule Rule, item git.WorkItem) bool {
	if len(rule.Sides) > 0 {
		matched := false
		for _, side := range rule.Sides {
			if item.Side == side {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(rule.Paths) == 0 {
		return true
	}
	for _, pattern := range rule.Paths {
		target := item.Path
		if !strings.Contains(pattern, "/") {
			target = path.Base(target)
		}
		matched, _ := path.Match(pattern, target) // validated at load
		if matched {
			return true
		}
	}
	return false
}

// Questions copies typed criteria into planner bindings. It returns no partial
// result for duplicate or invalid selections.
func Questions(ctx context.Context, selections []Selection) ([]contextplan.Binding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(selections) > maxSelections {
		return nil, ErrPackLimit
	}
	bindings := make([]contextplan.Binding, 0, len(selections))
	seen := make(map[string]bool, len(selections))
	selectedBytes := 0
	for _, selection := range selections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if selection.GroupID == "" || !validSelectionItem(selection.Item) ||
			selection.QuestionID != selection.Item.ID+"/"+selection.Rule.ID ||
			len(selection.QuestionID) > 128 || seen[selection.QuestionID] {
			return nil, ErrInvalidRule
		}
		weight := ruleBytes(selection.Rule)
		if weight > maxSelectedRuleBytes-selectedBytes {
			return nil, ErrPackLimit
		}
		selectedBytes += weight
		seen[selection.QuestionID] = true
		var question jev.Question
		switch selection.Rule.Type {
		case "noul":
			var criteria *jev.NoulCriteria
			if selection.Rule.NoulCriteria != nil {
				copy := *selection.Rule.NoulCriteria
				criteria = &copy
			}
			question = jev.NoulQuestion{Instructions: selection.Rule.Instructions, Criteria: criteria}
		case "choice":
			criteria := make(map[string]string, len(selection.Rule.ChoiceCriteria))
			for key, value := range selection.Rule.ChoiceCriteria {
				criteria[key] = value
			}
			question = jev.ChoiceQuestion{Instructions: selection.Rule.Instructions, Criteria: criteria}
		case "score":
			question = jev.ScoreQuestion{Instructions: selection.Rule.Instructions,
				Criteria: append([]string(nil), selection.Rule.ScoreCriteria...)}
		default:
			return nil, ErrInvalidRule
		}
		if err := jev.ValidateRequest(jev.Request{Model: "validation", State: "validation",
			Questions: map[string]jev.Question{selection.QuestionID: question}}); err != nil {
			return nil, ErrInvalidRule
		}
		bindings = append(bindings, contextplan.Binding{GroupID: selection.GroupID,
			WorkItemID: selection.Item.ID, QuestionID: selection.QuestionID, Question: question})
	}
	return bindings, nil
}

func ruleBytes(rule Rule) int {
	count := len(rule.ID) + len(rule.Instructions) + len(rule.Title) + len(rule.Message) + len(rule.Type)
	if rule.NoulCriteria != nil {
		count += len(rule.NoulCriteria.True) + len(rule.NoulCriteria.False)
	}
	for key, value := range rule.ChoiceCriteria {
		count += len(key) + len(value)
	}
	for _, value := range rule.ScoreCriteria {
		count += len(value)
	}
	for _, value := range rule.TriggerChoices {
		count += len(value)
	}
	for _, value := range rule.Paths {
		count += len(value)
	}
	return count
}
