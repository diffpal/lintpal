package di

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/diffpal/jevlint/internal/apps/jevlint/app"
	"github.com/diffpal/jevlint/internal/apps/jevlint/cli"
	"github.com/diffpal/jevlint/internal/apps/jevlint/git"
	"github.com/diffpal/jevlint/internal/apps/jevlint/jev"
	"github.com/diffpal/jevlint/internal/apps/jevlint/provider/systemone"
	"github.com/diffpal/jevlint/internal/apps/jevlint/report"
	"github.com/diffpal/jevlint/internal/apps/jevlint/rules"
	adkruntime "github.com/diffpal/jevlint/internal/apps/jevlint/runtime/adk"
)

const lifecycleTimeout = 15 * time.Second

// Lint creates a run-scoped Fx graph after CLI options are resolved. It returns
// an artifact only after lint and resource shutdown both succeed.
func Lint(ctx context.Context, dir string, options cli.Options) (report.Report, error) {
	return lintWithRuntime(ctx, dir, options, adkruntime.New())
}

func lintWithRuntime(ctx context.Context, dir string, options cli.Options, runtime *adkruntime.Runtime) (report.Report, error) {
	var linter *app.Linter
	graph := fx.New(
		fx.Supply(runtime),
		fx.Provide(func() cli.Options { return options }),
		fx.Provide(func() context.Context { return ctx }),
		fx.Provide(func() *http.Client { return http.DefaultClient }),
		fx.Provide(fx.Annotate(func() (*git.Repository, error) { return git.NewRepository(dir, git.Limits{}) }, fx.As(new(app.Comparer)))),
		fx.Provide(loadPack),
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
	err := graph.Start(startCtx)
	cancelStart()
	if err != nil {
		return report.Report{}, err
	}
	artifact, runErr := linter.Lint(ctx, app.Request{Base: options.Base, Head: options.Head, Model: options.Model, ProviderName: options.Provider, Limits: options.Limits})
	stopCtx, cancelStop := context.WithTimeout(context.Background(), lifecycleTimeout)
	stopErr := graph.Stop(stopCtx)
	cancelStop()
	if err := errors.Join(runErr, stopErr); err != nil {
		return report.Report{}, err
	}
	return artifact, nil
}

func loadPack(ctx context.Context, options cli.Options) (rules.Pack, error) {
	if options.Rules == "" {
		return rules.BuiltIn(), nil
	}
	file, err := os.Open(options.Rules)
	if err != nil {
		return rules.Pack{}, err
	}
	defer file.Close()
	return rules.LoadContext(ctx, file)
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
	return systemone.New(endpoint, client)
}
