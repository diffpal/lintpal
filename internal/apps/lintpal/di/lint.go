package di

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/diffpal/lintpal/internal/apps/lintpal/app"
	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
	"github.com/diffpal/lintpal/internal/apps/lintpal/provider/systemone"
	"github.com/diffpal/lintpal/internal/apps/lintpal/report"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rules"
	"github.com/diffpal/lintpal/internal/apps/lintpal/rulesource"
	adkruntime "github.com/diffpal/lintpal/internal/apps/lintpal/runtime/adk"
)

const lifecycleTimeout = 15 * time.Second

// Lint creates a run-scoped Fx graph after CLI options are resolved. It returns
// an artifact only after lint and resource shutdown both succeed.
func Lint(ctx context.Context, dir string, options cli.Options) (report.Report, error) {
	return lintWithRuntime(ctx, dir, options, adkruntime.New())
}

func lintWithRuntime(ctx context.Context, dir string, options cli.Options, runtime *adkruntime.Runtime) (report.Report, error) {
	pack, err := loadPack(ctx, dir, options)
	if err != nil {
		return report.Report{}, err
	}
	var linter *app.Linter
	graph := fx.New(
		fx.Supply(runtime, pack),
		fx.Provide(func() cli.Options { return options }),
		fx.Provide(func() context.Context { return ctx }),
		fx.Provide(func() *http.Client { return http.DefaultClient }),
		fx.Provide(fx.Annotate(func() (*git.Repository, error) { return git.NewRepository(dir, git.Limits{}) }, fx.As(new(app.Comparer)))),
		fx.Provide(fx.Annotate(newProvider, fx.As(new(jev.Provider)))),
		fx.Provide(func(runtime *adkruntime.Runtime) app.Observer { return runtime }),
		fx.Provide(app.NewObservedLinter),
		fx.Invoke(func(lifecycle fx.Lifecycle, runtime *adkruntime.Runtime) {
			lifecycle.Append(fx.Hook{OnStop: runtime.Shutdown})
		}),
		fx.Populate(&linter),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
	if err := graph.Err(); err != nil {
		return report.Report{}, err
	}
	startCtx, cancelStart := context.WithTimeout(ctx, lifecycleTimeout)
	err = graph.Start(startCtx)
	cancelStart()
	if err != nil {
		return report.Report{}, err
	}
	artifact, runErr := linter.Lint(ctx, app.Request{Base: options.Base, Head: options.Head, Uncommitted: options.Uncommitted, Model: options.Model, ProviderName: options.Provider,
		Include: options.Include, Exclude: options.Exclude, Limits: options.Limits})
	stopCtx, cancelStop := context.WithTimeout(context.Background(), lifecycleTimeout)
	stopErr := graph.Stop(stopCtx)
	cancelStop()
	if err := errors.Join(runErr, stopErr); err != nil {
		return report.Report{}, err
	}
	return artifact, nil
}

func loadPack(ctx context.Context, dir string, options cli.Options) (rules.Pack, error) {
	applyPolicy := func(pack rules.Pack) (rules.Pack, error) {
		var threshold *float64
		var severity *rules.Severity
		if options.RuleThresholdSet {
			threshold = &options.RuleThreshold
		}
		if options.RuleSeveritySet {
			severity = &options.RuleSeverity
		}
		return rules.WithOverrides(pack, threshold, severity)
	}
	root, err := rulesource.RepositoryRoot(ctx, dir)
	if err != nil {
		return rules.Pack{}, err
	}
	selectedRules := options.Rules
	if selectedRules == "" {
		selectedRules = filepath.Join(root, ".lintpal", "rules")
	}
	if strings.HasPrefix(options.Rules, "@") {
		return rules.Pack{}, cli.ErrInvalidOptions
	}
	selected, err := filepath.Abs(selectedRules)
	if err != nil {
		return rules.Pack{}, rulesource.ErrSource
	}
	pack, err := rules.LoadDirectory(ctx, selected)
	if err != nil {
		return rules.Pack{}, err
	}
	return applyPolicy(pack)
}

func newProvider(options cli.Options, client *http.Client) (*systemone.Provider, error) {
	var endpoint systemone.Endpoint
	switch options.Provider {
	case "jev":
		endpoint = systemone.TypeSafe()
	case "openrouter":
		endpoint = systemone.OpenRouter()
	case "custom":
		var err error
		endpoint, err = systemone.TrustedCustom(options.BaseURL, options.AuthTokenEnv)
		if err != nil {
			return nil, err
		}
	default:
		return nil, cli.ErrInvalidOptions
	}
	if options.CredentialResolved {
		return systemone.NewWithToken(endpoint, client, options.Credential)
	}
	return systemone.New(endpoint, client)
}
