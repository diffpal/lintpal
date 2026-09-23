package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/diffpal/jevlint/internal/apps/jevlint/contextplan"
	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
	"github.com/diffpal/jevlint/internal/apps/jevlint/report"
	"github.com/diffpal/jevlint/internal/apps/jevlint/rules"
)

var ErrInvalidRun = errors.New("invalid lint run")
var ErrReportLimit = errors.New("lint report size limit exceeded")

type Comparer interface {
	Compare(context.Context, string, string) (git.Result, error)
}

// Stage and Status are fixed observability dimensions. They never carry input.
type Stage string
type Status string

const (
	StageCompare  Stage  = "compare"
	StageAssemble Stage  = "assemble"
	StageSelect   Stage  = "select"
	StagePlan     Stage  = "plan"
	StageEvaluate Stage  = "evaluate"
	StageReport   Stage  = "report"
	StageWrite    Stage  = "write"
	StatusOK      Status = "ok"
	StatusError   Status = "error"
)

type Observer interface {
	Observe(Stage, Status, int, time.Duration)
}

type Limits struct {
	Context        contextplan.Limits
	Concurrency    int
	Timeout        time.Duration
	MaxReportBytes int
}

func (l Limits) normalized() (Limits, error) {
	if l.Concurrency < 0 || l.Concurrency > 16 || l.Timeout < 0 || l.Timeout > 10*time.Minute ||
		l.MaxReportBytes < 0 || l.MaxReportBytes > 64<<20 {
		return Limits{}, ErrInvalidRun
	}
	if l.Concurrency == 0 {
		l.Concurrency = 4
	}
	if l.Timeout == 0 {
		l.Timeout = 2 * time.Minute
	}
	if l.MaxReportBytes == 0 {
		l.MaxReportBytes = 16 << 20
	}
	return l, nil
}

type Request struct {
	Base         string
	Head         string
	Model        string
	ProviderName string
	Limits       Limits
}

// Linter is the pure application boundary; process configuration belongs to the CLI.
type Linter struct {
	comparer Comparer
	provider jev.Provider
	pack     rules.Pack
	observer Observer
}

func NewLinter(comparer Comparer, provider jev.Provider, pack rules.Pack) (*Linter, error) {
	return NewObservedLinter(comparer, provider, pack, nil)
}

func NewObservedLinter(comparer Comparer, provider jev.Provider, pack rules.Pack, observer Observer) (*Linter, error) {
	if comparer == nil || provider == nil || len(pack.Rules()) == 0 {
		return nil, ErrInvalidRun
	}
	return &Linter{comparer: comparer, provider: provider, pack: pack, observer: observer}, nil
}

func (l *Linter) observe(stage Stage, started time.Time, count int, err error) {
	if l.observer == nil {
		return
	}
	status := StatusOK
	if err != nil {
		status = StatusError
	}
	l.observer.Observe(stage, status, count, time.Since(started))
}

func (l *Linter) Lint(parent context.Context, request Request) (report.Report, error) {
	if l == nil || l.comparer == nil || l.provider == nil || request.Base == "" || request.Head == "" ||
		request.Model == "" || request.ProviderName == "" {
		return report.Report{}, ErrInvalidRun
	}
	limits, err := request.Limits.normalized()
	if err != nil {
		return report.Report{}, err
	}
	if err := parent.Err(); err != nil {
		return report.Report{}, err
	}
	ctx, cancel := context.WithTimeout(parent, limits.Timeout)
	defer cancel()
	started := time.Now()
	result, err := l.comparer.Compare(ctx, request.Base, request.Head)
	l.observe(StageCompare, started, len(result.Items), err)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "compare", err)
	}
	started = time.Now()
	groups, err := contextplan.Assemble(ctx, result, limits.Context)
	l.observe(StageAssemble, started, len(groups), err)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "assemble", err)
	}
	started = time.Now()
	selections, err := rules.Select(ctx, l.pack, groups)
	l.observe(StageSelect, started, len(selections), err)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "select", err)
	}
	bindings, err := rules.Questions(ctx, selections)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "questions", err)
	}
	started = time.Now()
	batches, err := contextplan.Plan(ctx, groups, bindings, request.Model, limits.Context)
	l.observe(StagePlan, started, len(batches), err)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "plan", err)
	}
	stats := report.Stats{WorkItems: len(result.Items), Skipped: len(result.Skips), Groups: len(groups), Batches: len(batches), Questions: len(bindings)}
	started = time.Now()
	results, err := l.evaluate(ctx, batches, selections, limits.Concurrency)
	l.observe(StageEvaluate, started, len(results), err)
	if err != nil {
		return report.Report{}, stageError(parent, ctx, "evaluate", err)
	}
	decisions := make([]rules.Decision, 0)
	model := request.Model
	for index, output := range results {
		if index == 0 {
			model = output.response.Model
		}
		if output.response.Model != model {
			return report.Report{}, ErrInvalidRun
		}
		if output.response.Usage.InputTokens > math.MaxInt-stats.InputTokens || output.response.Usage.OutputTokens > math.MaxInt-stats.OutputTokens {
			return report.Report{}, ErrInvalidRun
		}
		stats.InputTokens += output.response.Usage.InputTokens
		stats.OutputTokens += output.response.Usage.OutputTokens
		decisions = append(decisions, output.decisions...)
	}
	if err := ctx.Err(); err != nil {
		return report.Report{}, stageError(parent, ctx, "run", err)
	}
	started = time.Now()
	artifact, err := report.New(result, decisions, request.ProviderName, model, stats)
	if err != nil {
		l.observe(StageReport, started, 0, err)
		return report.Report{}, fmt.Errorf("report: %w", err)
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		l.observe(StageReport, started, 0, err)
		return report.Report{}, report.ErrInvalidReport
	}
	if len(data)+1 > limits.MaxReportBytes {
		l.observe(StageReport, started, 0, ErrReportLimit)
		return report.Report{}, ErrReportLimit
	}
	if err := ctx.Err(); err != nil {
		l.observe(StageReport, started, 0, err)
		return report.Report{}, stageError(parent, ctx, "run", err)
	}
	l.observe(StageReport, started, len(artifact.Diagnostics), nil)
	return artifact, nil
}

type batchResult struct {
	response  jev.Response
	decisions []rules.Decision
}

func (l *Linter) evaluate(ctx context.Context, batches []contextplan.Batch, selections []rules.Selection, concurrency int) ([]batchResult, error) {
	results := make([]batchResult, len(batches))
	if len(batches) == 0 {
		return results, nil
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	workers := concurrency
	if workers > len(batches) {
		workers = len(batches)
	}
	jobs := make(chan int)
	errorsByBatch := make([]error, len(batches))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				response, err := l.provider.Evaluate(workCtx, batches[index].Request)
				if err == nil {
					results[index].decisions, err = rules.Decide(workCtx, batches[index], selections, response)
				}
				if err != nil {
					errorsByBatch[index] = err
					cancel()
					continue
				}
				results[index].response = response
			}
		}()
	}
send:
	for index := range batches {
		select {
		case <-workCtx.Done():
			break send
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, err := range errorsByBatch {
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	for _, err := range errorsByBatch {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func stageError(parent, run context.Context, stage string, err error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if run.Err() != nil {
		return run.Err()
	}
	return fmt.Errorf("%s: %w", stage, err)
}
