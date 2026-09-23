package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/diffpal/lintpal/internal/apps/lintpal/cli"
	"github.com/diffpal/lintpal/internal/apps/lintpal/di"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root := cli.NewRoot(func(ctx context.Context, options cli.Options, writer io.Writer, stderr io.Writer) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		return di.ExecuteLint(ctx, dir, options, writer, stderr)
	}, version)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	code := cli.ExitCode(err)
	if code != 0 {
		fmt.Fprintln(stderr, cli.ExitMessage(code))
	}
	return code
}
