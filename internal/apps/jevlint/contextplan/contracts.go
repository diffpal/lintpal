// Package contextplan assembles bounded committed-source context and plans
// typed questions without invoking a provider.
package contextplan

import (
	"errors"

	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
)

var (
	ErrInvalidInput  = errors.New("invalid context input")
	ErrInvalidLimits = errors.New("invalid context limits")
	ErrLimit         = errors.New("context limit exceeded")
)

// Limits are local preflight caps. Zero selects a finite default; callers may
// lower a cap but cannot raise it above the hard maximum. ByteBudget is a
// conservative upper estimate of tokens: one serialized UTF-8 byte counts as
// one token. It is not the provider's tokenizer.
type Limits struct {
	MaxStateBytes        int
	MaxTotalStateBytes   int
	MaxRequestBytes      int
	ByteBudget           int
	MaxItems             int
	MaxGroups            int
	MaxQuestions         int
	MaxQuestionsPerBatch int
	MaxPlanBytes         int
}

const (
	defaultStateBytes        = 20_000
	defaultTotalStateBytes   = 16 << 20
	defaultRequestBytes      = 24_000
	defaultByteBudget        = 24_000
	defaultMaxItems          = 10_000
	defaultMaxGroups         = 10_000
	defaultMaxQuestions      = 100_000
	defaultQuestionsPerBatch = 128
	defaultPlanBytes         = 16 << 20

	hardStateBytes        = 28_000
	hardTotalStateBytes   = 128 << 20
	hardRequestBytes      = 30_000 // below the 1 MiB System One transport cap
	hardByteBudget        = 30_000 // below the portable 32K context target
	hardMaxItems          = 100_000
	hardMaxGroups         = 100_000
	hardMaxQuestions      = 100_000
	hardQuestionsPerBatch = 1024
	hardPlanBytes         = 128 << 20
)

func (l Limits) normalized() (Limits, error) {
	fields := []*int{&l.MaxStateBytes, &l.MaxTotalStateBytes, &l.MaxRequestBytes, &l.ByteBudget,
		&l.MaxItems, &l.MaxGroups, &l.MaxQuestions, &l.MaxQuestionsPerBatch, &l.MaxPlanBytes}
	defaults := []int{defaultStateBytes, defaultTotalStateBytes, defaultRequestBytes, defaultByteBudget,
		defaultMaxItems, defaultMaxGroups, defaultMaxQuestions, defaultQuestionsPerBatch, defaultPlanBytes}
	hard := []int{hardStateBytes, hardTotalStateBytes, hardRequestBytes, hardByteBudget,
		hardMaxItems, hardMaxGroups, hardMaxQuestions, hardQuestionsPerBatch, hardPlanBytes}
	for i, field := range fields {
		if *field < 0 || *field > hard[i] {
			return Limits{}, ErrInvalidLimits
		}
		if *field == 0 {
			*field = defaults[i]
		}
	}
	return l, nil
}

// Group retains the exact work-item anchors represented by State.
type Group struct {
	ID    string
	State string
	Items []git.WorkItem
}

// Binding ties one typed decision question to one anchored work item.
type Binding struct {
	GroupID    string
	WorkItemID string
	QuestionID string
	Question   jev.Question
}

// Batch is one request against one state. ItemByQuestion maps answer IDs back
// to immutable Git work-item IDs for later diagnostic anchoring.
type Batch struct {
	GroupIDs       []string
	Request        jev.Request
	ItemByQuestion map[string]string
}
