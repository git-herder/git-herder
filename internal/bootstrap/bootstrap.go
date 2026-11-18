// Пакет bootstrap — точка сборки: регистрация команд, Logger, Execute.
package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/cli"
	_ "github.com/git-herder/git-herder/internal/cli/clone"
	_ "github.com/git-herder/git-herder/internal/cli/fetch"
	_ "github.com/git-herder/git-herder/internal/cli/merge"
	_ "github.com/git-herder/git-herder/internal/cli/publish"
	_ "github.com/git-herder/git-herder/internal/cli/pull"
	_ "github.com/git-herder/git-herder/internal/cli/push"
	_ "github.com/git-herder/git-herder/internal/cli/remote"
	reportcmd "github.com/git-herder/git-herder/internal/cli/report"
	_ "github.com/git-herder/git-herder/internal/cli/switch"
	_ "github.com/git-herder/git-herder/internal/cli/sync"
	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/version"
)

// Run запускает CLI и завершает процесс с кодом по спеке.
func Run() {
	exitFn(RunWithArgs(os.Args[1:]))
}

// RunWithArgs выполняет CLI и возвращает код выхода (для тестов).
func RunWithArgs(args []string) (code int) {
	defer closeActiveTrace()
	leave := logging.Enter("args", args)
	defer func() { leave("code", code) }()

	root, err := cli.NewRootCommand()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	prepared, call, err := cli.PrepareProxyArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	if call != nil && call.Version {
		fmt.Printf("githerd version %s\n", version.Semver)
		return 0
	}
	if call != nil && call.Help {
		prepared = []string{call.Command, "--help"}
	}
	root.SetArgs(reportcmd.NormalizeArgs(prepared))
	ctx := context.Background()
	if call != nil {
		ctx = cli.WithProxyCall(ctx, *call)
	}
	if err := root.ExecuteContext(ctx); err != nil {
		code = cli.ExitCodeFromError(err)
		if cli.ActiveRuntime != nil && cli.ActiveRuntime.Trace != nil {
			log := cli.ActiveRuntime.Trace.Logger("cli.bootstrap")
			cli.ActiveRuntime.Trace.LogError(log, err)
		}
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		return code
	}
	return 0
}

func closeActiveTrace() {
	if cli.ActiveRuntime != nil && cli.ActiveRuntime.Trace != nil {
		_ = cli.ActiveRuntime.Trace.Close()
	}
}

// RootForTest экспортирует root для тестов (без os.Exit).
func RootForTest() (*cobra.Command, error) {
	return cli.NewRootCommand()
}
