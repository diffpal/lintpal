package contextplan

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

type groupInfo struct {
	group Group
	rank  int
	items map[string]int
}

type stateBucket struct {
	state    string
	bindings []Binding
}

// Plan creates bounded System One requests. Every binding appears once, and
// equal state strings share a request whenever the complete request fits.
func Plan(ctx context.Context, groups []Group, bindings []Binding, model string, limits Limits) ([]Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		return nil, ErrInvalidInput
	}
	if len(groups) > l.MaxGroups {
		return nil, ErrLimit
	}
	if len(bindings) > l.MaxQuestions {
		return nil, ErrLimit
	}
	ordered := append([]Group(nil), groups...)
	totalState := 0
	for _, group := range ordered {
		if group.ID == "" || len(group.Items) == 0 || group.ID != groupID(group.Items) || group.State == "" {
			return nil, ErrInvalidInput
		}
		if len(group.State) > l.MaxStateBytes || len(group.State) > l.ByteBudget {
			return nil, ErrLimit
		}
		if len(group.State) > l.MaxTotalStateBytes-totalState {
			return nil, ErrLimit
		}
		totalState += len(group.State)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].Items[0], ordered[j].Items[0]
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
		return ordered[i].ID < ordered[j].ID
	})
	info := make(map[string]groupInfo, len(ordered))
	seenItems := make(map[string]bool)
	for rank, group := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := info[group.ID]; exists {
			return nil, ErrInvalidInput
		}
		items := make(map[string]int, len(group.Items))
		for i, item := range group.Items {
			if !validItem(item) || seenItems[item.ID] {
				return nil, ErrInvalidInput
			}
			if len(seenItems) >= l.MaxItems {
				return nil, ErrLimit
			}
			items[item.ID] = i
			seenItems[item.ID] = true
		}
		info[group.ID] = groupInfo{group: group, rank: rank, items: items}
	}
	seenQuestions := make(map[string]bool, len(bindings))
	orderedBindings := append([]Binding(nil), bindings...)
	for i := range orderedBindings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		binding := &orderedBindings[i]
		group, ok := info[binding.GroupID]
		if !ok || binding.QuestionID == "" || seenQuestions[binding.QuestionID] {
			return nil, ErrInvalidInput
		}
		if _, ok := group.items[binding.WorkItemID]; !ok {
			return nil, ErrInvalidInput
		}
		seenQuestions[binding.QuestionID] = true
		cloned, ok := cloneQuestion(binding.Question)
		if !ok {
			return nil, ErrInvalidInput
		}
		binding.Question = cloned
		if err := jev.ValidateRequest(jev.Request{Model: model, State: group.group.State,
			Questions: map[string]jev.Question{binding.QuestionID: binding.Question}}); err != nil {
			return nil, ErrInvalidInput
		}
	}
	sort.Slice(orderedBindings, func(i, j int) bool {
		a, b := orderedBindings[i], orderedBindings[j]
		if info[a.GroupID].rank != info[b.GroupID].rank {
			return info[a.GroupID].rank < info[b.GroupID].rank
		}
		if info[a.GroupID].items[a.WorkItemID] != info[b.GroupID].items[b.WorkItemID] {
			return info[a.GroupID].items[a.WorkItemID] < info[b.GroupID].items[b.WorkItemID]
		}
		return a.QuestionID < b.QuestionID
	})
	buckets := make([]*stateBucket, 0, len(groups))
	byState := make(map[string]*stateBucket)
	for _, binding := range orderedBindings {
		state := info[binding.GroupID].group.State
		bucket := byState[state]
		if bucket == nil {
			bucket = &stateBucket{state: state}
			byState[state] = bucket
			buckets = append(buckets, bucket)
		}
		bucket.bindings = append(bucket.bindings, binding)
	}
	batches := make([]Batch, 0, len(bindings))
	planBytes := 0
	for _, bucket := range buckets {
		var current Batch
		for _, binding := range bucket.bindings {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if current.Request.Questions == nil {
				current = Batch{Request: jev.Request{Model: model, State: bucket.state,
					Questions: make(map[string]jev.Question)}, ItemByQuestion: make(map[string]string)}
			}
			if len(current.Request.Questions) >= l.MaxQuestionsPerBatch {
				var err error
				batches, planBytes, err = appendBatch(batches, current, planBytes, l)
				if err != nil {
					return nil, err
				}
				current = Batch{Request: jev.Request{Model: model, State: bucket.state,
					Questions: make(map[string]jev.Question)}, ItemByQuestion: make(map[string]string)}
			}
			current.Request.Questions[binding.QuestionID] = binding.Question
			current.ItemByQuestion[binding.QuestionID] = binding.WorkItemID
			if !contains(current.GroupIDs, binding.GroupID) {
				current.GroupIDs = append(current.GroupIDs, binding.GroupID)
			}
			fits, err := requestFits(current.Request, l)
			if err != nil {
				return nil, err
			}
			if !fits {
				delete(current.Request.Questions, binding.QuestionID)
				delete(current.ItemByQuestion, binding.QuestionID)
				if len(current.Request.Questions) == 0 {
					return nil, ErrLimit
				}
				if !groupUsed(current, binding.GroupID, info) {
					current.GroupIDs = current.GroupIDs[:len(current.GroupIDs)-1]
				}
				batches, planBytes, err = appendBatch(batches, current, planBytes, l)
				if err != nil {
					return nil, err
				}
				current = Batch{GroupIDs: []string{binding.GroupID},
					Request: jev.Request{Model: model, State: bucket.state,
						Questions: map[string]jev.Question{binding.QuestionID: binding.Question}},
					ItemByQuestion: map[string]string{binding.QuestionID: binding.WorkItemID}}
				fits, err = requestFits(current.Request, l)
				if err != nil {
					return nil, err
				}
				if !fits {
					return nil, ErrLimit
				}
			}
		}
		if len(current.Request.Questions) > 0 {
			var err error
			batches, planBytes, err = appendBatch(batches, current, planBytes, l)
			if err != nil {
				return nil, err
			}
		}
	}
	return batches, nil
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func groupUsed(batch Batch, groupID string, info map[string]groupInfo) bool {
	for _, itemID := range batch.ItemByQuestion {
		if _, ok := info[groupID].items[itemID]; ok {
			return true
		}
	}
	return false
}

func appendBatch(batches []Batch, batch Batch, planBytes int, limits Limits) ([]Batch, int, error) {
	body, err := requestBody(batch.Request)
	if err != nil {
		return nil, 0, err
	}
	if len(body) > limits.MaxPlanBytes-planBytes {
		return nil, 0, ErrLimit
	}
	return append(batches, batch), planBytes + len(body), nil
}

func requestFits(request jev.Request, limits Limits) (bool, error) {
	body, err := requestBody(request)
	if err != nil {
		return false, err
	}
	return len(body) <= limits.MaxRequestBytes && len(body) <= limits.ByteBudget, nil
}

// requestBody mirrors the System One transport's request JSON fields so the
// byte cap includes escaping, IDs, instructions, criteria and metadata.
func requestBody(request jev.Request) ([]byte, error) {
	questions := make(map[string]any, len(request.Questions))
	for id, question := range request.Questions {
		questions[id] = wireQuestion(question)
	}
	body, err := json.Marshal(struct {
		State     any            `json:"state"`
		Model     string         `json:"model"`
		Questions map[string]any `json:"questions"`
	}{State: request.State, Model: request.Model, Questions: questions})
	if err != nil {
		return nil, ErrInvalidInput
	}
	return body, nil
}

func wireQuestion(question jev.Question) any {
	switch q := question.(type) {
	case jev.NoulQuestion:
		wire := map[string]any{"type": "noul", "instructions": q.Instructions}
		if q.Criteria != nil {
			wire["criteria"] = map[string]string{"true": q.Criteria.True, "false": q.Criteria.False}
		}
		return wire
	case jev.ChoiceQuestion:
		return map[string]any{"type": "choice", "instructions": q.Instructions, "criteria": q.Criteria}
	case jev.ScoreQuestion:
		return map[string]any{"type": "score", "instructions": q.Instructions, "criteria": q.Criteria}
	default:
		return nil
	}
}

func cloneQuestion(question jev.Question) (jev.Question, bool) {
	switch q := question.(type) {
	case jev.NoulQuestion:
		if q.Criteria != nil {
			criteria := *q.Criteria
			q.Criteria = &criteria
		}
		return q, true
	case *jev.NoulQuestion:
		if q == nil {
			return nil, false
		}
		return cloneQuestion(*q)
	case jev.ChoiceQuestion:
		criteria := make(map[string]string, len(q.Criteria))
		for key, value := range q.Criteria {
			criteria[key] = value
		}
		q.Criteria = criteria
		return q, true
	case *jev.ChoiceQuestion:
		if q == nil {
			return nil, false
		}
		return cloneQuestion(*q)
	case jev.ScoreQuestion:
		q.Criteria = append([]string(nil), q.Criteria...)
		return q, true
	case *jev.ScoreQuestion:
		if q == nil {
			return nil, false
		}
		return cloneQuestion(*q)
	default:
		return nil, false
	}
}
