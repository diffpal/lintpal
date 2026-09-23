// Package eval evaluates frozen examples with typed, local answers. It does
// not call a provider and its counts do not measure live model accuracy.
package eval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/diffpal/lintpal/internal/apps/lintpal/contextplan"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
)

const SchemaVersion = "lintpal.eval.corpus.v1"
const MaxCorpusBytes = 256 << 10

var ErrInvalidCorpus = errors.New("invalid evaluation corpus")

type Case struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Line        int     `json:"line"`
	Source      string  `json:"source"`
	RuleID      string  `json:"rule_id"`
	Truth       string  `json:"truth"`
	Probability float64 `json:"probability"`
}

type Corpus struct {
	SchemaVersion string `json:"schema_version"`
	Cases         []Case `json:"cases"`
}

type Outcome struct {
	ID          string `json:"id"`
	RuleID      string `json:"rule_id"`
	Truth       string `json:"truth"`
	Predicted   bool   `json:"predicted"`
	Diagnostics int    `json:"diagnostics"`
}

type Summary struct {
	SchemaVersion string    `json:"schema_version"`
	CorpusVersion string    `json:"corpus_version"`
	Cases         []Outcome `json:"cases"`
	TruePositive  int       `json:"true_positive"`
	TrueNegative  int       `json:"true_negative"`
	FalsePositive int       `json:"false_positive"`
	FalseNegative int       `json:"false_negative"`
}

func Load(reader io.Reader) (Corpus, error) {
	if reader == nil {
		return Corpus{}, ErrInvalidCorpus
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxCorpusBytes+1))
	if err != nil || len(data) > MaxCorpusBytes {
		return Corpus{}, ErrInvalidCorpus
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var corpus Corpus
	if err := decoder.Decode(&corpus); err != nil || decoder.Decode(new(any)) != io.EOF ||
		corpus.SchemaVersion != SchemaVersion || len(corpus.Cases) == 0 || len(corpus.Cases) > 128 {
		return Corpus{}, ErrInvalidCorpus
	}
	knownRules := make(map[string]bool)
	for _, rule := range rules.BuiltIn().Rules() {
		knownRules[rule.ID] = true
	}
	seen := make(map[string]bool)
	for _, c := range corpus.Cases {
		if c.ID == "" || len(c.ID) > 64 || seen[c.ID] || c.Path == "" || len(c.Path) > 256 ||
			c.Line < 1 || c.Line > 100_000 || c.Source == "" || len(c.Source) > 8<<10 ||
			!knownRules[c.RuleID] || c.Truth != "safe" && c.Truth != "buggy" ||
			math.IsNaN(c.Probability) || math.IsInf(c.Probability, 0) || c.Probability < 0 || c.Probability > 1 {
			return Corpus{}, ErrInvalidCorpus
		}
		seen[c.ID] = true
	}
	return corpus, nil
}

// RunOffline executes the real selection, decision, and report path against
// frozen examples. Probabilities are fixture inputs, not model predictions.
func RunOffline(ctx context.Context, corpus Corpus) (Summary, []report.Report, error) {
	if ctx == nil || corpus.SchemaVersion != SchemaVersion || len(corpus.Cases) == 0 {
		return Summary{}, nil, ErrInvalidCorpus
	}
	ordered := append([]Case(nil), corpus.Cases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	summary := Summary{SchemaVersion: "lintpal.eval.summary.v1", CorpusVersion: corpus.SchemaVersion,
		Cases: make([]Outcome, 0, len(ordered))}
	reports := make([]report.Report, 0, len(ordered))
	for _, c := range ordered {
		if err := ctx.Err(); err != nil {
			return Summary{}, nil, err
		}
		artifact, err := runCase(ctx, c)
		if err != nil {
			return Summary{}, nil, err
		}
		predicted := false
		for _, diagnostic := range artifact.Diagnostics {
			if diagnostic.RuleID == c.RuleID {
				predicted = true
			}
		}
		summary.Cases = append(summary.Cases, Outcome{ID: c.ID, RuleID: c.RuleID, Truth: c.Truth,
			Predicted: predicted, Diagnostics: len(artifact.Diagnostics)})
		switch {
		case c.Truth == "buggy" && predicted:
			summary.TruePositive++
		case c.Truth == "buggy":
			summary.FalseNegative++
		case predicted:
			summary.FalsePositive++
		default:
			summary.TrueNegative++
		}
		reports = append(reports, artifact)
	}
	return summary, reports, nil
}

func runCase(ctx context.Context, c Case) (report.Report, error) {
	id := sha256.Sum256([]byte(c.ID))
	item := git.WorkItem{ID: hex.EncodeToString(id[:]), NewPath: c.Path, Path: c.Path,
		Side: git.Right, Hunk: 1, StartLine: c.Line, EndLine: c.Line}
	groupID := "fixture-" + c.ID
	groups := []contextplan.Group{{ID: groupID, State: c.Source, Items: []git.WorkItem{item}}}
	selections, err := rules.Select(ctx, rules.BuiltIn(), groups)
	if err != nil {
		return report.Report{}, err
	}
	bindings, err := rules.Questions(ctx, selections)
	if err != nil {
		return report.Report{}, err
	}
	batch := contextplan.Batch{GroupIDs: []string{groupID}, Request: jev.Request{Model: "jev-offline",
		State: c.Source, Questions: make(map[string]jev.Question)}, ItemByQuestion: make(map[string]string)}
	answers := make(map[string]jev.Answer)
	for _, binding := range bindings {
		batch.Request.Questions[binding.QuestionID] = binding.Question
		batch.ItemByQuestion[binding.QuestionID] = item.ID
		probability := 0.01
		if strings.HasSuffix(binding.QuestionID, "/"+c.RuleID) {
			probability = c.Probability
		}
		answers[binding.QuestionID] = jev.NoulAnswer{Probability: probability}
	}
	decisions, err := rules.Decide(ctx, batch, selections, jev.Response{Model: batch.Request.Model, Answers: answers})
	if err != nil {
		return report.Report{}, err
	}
	result := git.Result{Revisions: git.Revisions{Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40),
		MergeBase: strings.Repeat("a", 40)}, Items: []git.WorkItem{item}}
	stats := report.Stats{WorkItems: 1, Groups: 1, Batches: 1, Questions: len(bindings)}
	return report.New(result, decisions, "fixture", batch.Request.Model, stats)
}
