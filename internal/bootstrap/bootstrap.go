// Пакет bootstrap — точка сборки: регистрация команд, Logger, Execute.
package bootstrap

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/cli"
	reportcmd "github.com/Snowbridge/git-herder/internal/cli/report"
	"github.com/Snowbridge/git-herder/internal/diag/logging"
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
	root.SetArgs(reportcmd.NormalizeArgs(args))
	if err := root.Execute(); err != nil {
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
