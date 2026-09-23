package rules

import (
	"context"
	"errors"
	"io"
	"math"
	"path"
	"regexp"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"go.yaml.in/yaml/v3"
)

var ErrInvalidRule = errors.New("invalid rule")

const maxRules = 256
const maxInstructions = 2048
const maxTitle = 120
const maxMessage = 1024
const maxPattern = 256
const maxCriteriaText = 512

var ruleID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

type Severity string

const (
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

type Rule struct {
	ID             string
	Type           string
	Instructions   string
	NoulCriteria   *jev.NoulCriteria
	ChoiceCriteria map[string]string
	ScoreCriteria  []string
	TriggerChoices []string
	Threshold      float64
	Severity       Severity
	Title          string
	Message        string
	Paths          []string
	Sides          []git.Side
}

type Pack struct {
	rules []Rule
}

// Rules returns independent copies; changing them cannot alter a loaded pack.
func (p Pack) Rules() []Rule {
	out := make([]Rule, len(p.rules))
	for i, rule := range p.rules {
		out[i] = cloneRule(rule)
	}
	return out
}

// Load parses and validates one untrusted declarative rule pack.
func Load(reader io.Reader) (Pack, error) {
	return LoadContext(context.Background(), reader)
}

// LoadContext adds cancellation to bounded rule loading for CI callers.
func LoadContext(ctx context.Context, reader io.Reader) (Pack, error) {
	raw, err := parseContext(ctx, reader)
	if err != nil {
		return Pack{}, err
	}
	if err := ctx.Err(); err != nil {
		return Pack{}, err
	}
	pack, err := compilePack(raw)
	if err != nil {
		return Pack{}, err
	}
	if err := ctx.Err(); err != nil {
		return Pack{}, err
	}
	return pack, nil
}

func compilePack(raw rawPack) (Pack, error) {
	if raw.Schema != schemaV1 {
		return Pack{}, ErrUnsupportedSchema
	}
	if len(raw.Rules) == 0 {
		return Pack{}, ErrInvalidPack
	}
	if len(raw.Rules) > maxRules {
		return Pack{}, ErrPackLimit
	}
	pack := Pack{rules: make([]Rule, 0, len(raw.Rules))}
	seen := make(map[string]bool, len(raw.Rules))
	for _, candidate := range raw.Rules {
		if seen[candidate.ID] {
			return Pack{}, ErrInvalidRule
		}
		rule, err := compileRule(candidate)
		if err != nil {
			return Pack{}, err
		}
		seen[rule.ID] = true
		pack.rules = append(pack.rules, rule)
	}
	return pack, nil
}

func compileRule(raw rawRule) (Rule, error) {
	if !ruleID.MatchString(raw.ID) || !boundedText(raw.Instructions, maxInstructions) ||
		!boundedText(raw.Title, maxTitle) || !boundedText(raw.Message, maxMessage) ||
		raw.Threshold == nil || math.IsNaN(*raw.Threshold) || math.IsInf(*raw.Threshold, 0) ||
		*raw.Threshold < 0 || *raw.Threshold > 1 || len(raw.Paths) > 32 || len(raw.Sides) > 2 {
		return Rule{}, ErrInvalidRule
	}
	rule := Rule{ID: raw.ID, Type: raw.Type, Instructions: raw.Instructions,
		Threshold: *raw.Threshold, Severity: Severity(raw.Severity), Title: raw.Title,
		Message: raw.Message, Paths: append([]string(nil), raw.Paths...)}
	switch rule.Severity {
	case Low, Medium, High, Critical:
	default:
		return Rule{}, ErrInvalidRule
	}
	seenPaths := make(map[string]bool, len(raw.Paths))
	for _, pattern := range raw.Paths {
		if pattern == "" || len(pattern) > maxPattern || strings.Contains(pattern, "\\") ||
			strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "..") || seenPaths[pattern] {
			return Rule{}, ErrInvalidRule
		}
		if _, err := path.Match(pattern, "example.go"); err != nil {
			return Rule{}, ErrInvalidRule
		}
		seenPaths[pattern] = true
	}
	seenSides := make(map[git.Side]bool, len(raw.Sides))
	for _, value := range raw.Sides {
		side := git.Side(value)
		if side != git.Left && side != git.Right || seenSides[side] {
			return Rule{}, ErrInvalidRule
		}
		seenSides[side] = true
		rule.Sides = append(rule.Sides, side)
	}
	switch raw.Type {
	case "noul":
		if len(raw.TriggerChoices) != 0 {
			return Rule{}, ErrInvalidRule
		}
		if raw.Criteria.Kind != 0 {
			if raw.Criteria.Kind != yaml.MappingNode {
				return Rule{}, ErrInvalidRule
			}
			var criteria map[string]string
			if err := raw.Criteria.Decode(&criteria); err != nil || len(criteria) != 2 ||
				!boundedText(criteria["true"], maxCriteriaText) || !boundedText(criteria["false"], maxCriteriaText) {
				return Rule{}, ErrInvalidRule
			}
			rule.NoulCriteria = &jev.NoulCriteria{True: criteria["true"], False: criteria["false"]}
		}
	case "choice":
		if raw.Criteria.Kind != yaml.MappingNode {
			return Rule{}, ErrInvalidRule
		}
		var criteria map[string]string
		if err := raw.Criteria.Decode(&criteria); err != nil || len(criteria) < 2 || len(criteria) > 255 ||
			len(raw.TriggerChoices) == 0 || len(raw.TriggerChoices) > len(criteria) {
			return Rule{}, ErrInvalidRule
		}
		for key, value := range criteria {
			if !ruleID.MatchString(key) || !boundedText(value, maxCriteriaText) {
				return Rule{}, ErrInvalidRule
			}
		}
		seen := make(map[string]bool, len(raw.TriggerChoices))
		for _, trigger := range raw.TriggerChoices {
			if _, ok := criteria[trigger]; !ok || seen[trigger] {
				return Rule{}, ErrInvalidRule
			}
			seen[trigger] = true
		}
		rule.ChoiceCriteria = criteria
		rule.TriggerChoices = append([]string(nil), raw.TriggerChoices...)
	case "score":
		if raw.Criteria.Kind != yaml.SequenceNode || len(raw.TriggerChoices) != 0 {
			return Rule{}, ErrInvalidRule
		}
		var criteria []string
		if err := raw.Criteria.Decode(&criteria); err != nil || len(criteria) < 2 || len(criteria) > 10 {
			return Rule{}, ErrInvalidRule
		}
		for _, level := range criteria {
			if !boundedText(level, maxCriteriaText) {
				return Rule{}, ErrInvalidRule
			}
		}
		rule.ScoreCriteria = criteria
	default:
		return Rule{}, ErrInvalidRule
	}
	return rule, nil
}

func boundedText(value string, limit int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= limit && !strings.ContainsRune(value, 0)
}

func cloneRule(rule Rule) Rule {
	if rule.NoulCriteria != nil {
		criteria := *rule.NoulCriteria
		rule.NoulCriteria = &criteria
	}
	if rule.ChoiceCriteria != nil {
		criteria := make(map[string]string, len(rule.ChoiceCriteria))
		for key, value := range rule.ChoiceCriteria {
			criteria[key] = value
		}
		rule.ChoiceCriteria = criteria
	}
	rule.ScoreCriteria = append([]string(nil), rule.ScoreCriteria...)
	rule.TriggerChoices = append([]string(nil), rule.TriggerChoices...)
	rule.Paths = append([]string(nil), rule.Paths...)
	rule.Sides = append([]git.Side(nil), rule.Sides...)
	return rule
}

// BuiltIn uses the same compiler as a repository-provided pack.
func BuiltIn() Pack {
	threshold := 0.95
	pack, err := compilePack(rawPack{Schema: schemaV1, Rules: []rawRule{
		{ID: "security.shell-injection", Type: "noul",
			Instructions: "Do these changed Go lines introduce untrusted data into command execution without safe argument separation?",
			Threshold:    &threshold, Severity: string(High), Title: "Possible shell injection",
			Message: "Changed code may pass untrusted data to a shell command.", Paths: []string{"*.go"}},
		{ID: "correctness.ignored-error", Type: "noul",
			Instructions: "Do these changed Go lines ignore a returned error that could change correctness or safety?",
			Threshold:    &threshold, Severity: string(Medium), Title: "Possible ignored error",
			Message: "Changed code may ignore a meaningful error.", Paths: []string{"*.go"}},
	}})
	if err != nil {
		panic("invalid built-in rule pack")
	}
	return pack
}
