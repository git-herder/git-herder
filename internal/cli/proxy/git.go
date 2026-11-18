// Запуск git для прокси-команды через GitCliRunner.
package proxy

import (
	"context"

	"github.com/git-herder/git-herder/internal/gitexec"
)

// GitRunner выполняет один процесс git и возвращает его вывод.
type GitRunner interface {
	Run(ctx context.Context, dir string, args []string) (gitexec.Result, error)
}

type hostGit struct{}

func (hostGit) Run(ctx context.Context, dir string, args []string) (gitexec.Result, error) {
	return gitexec.Run(ctx, dir, args)
}

var runner GitRunner = hostGit{}

// InstallRunnerForTest подменяет процесс git. Возвращает восстановление.
func InstallRunnerForTest(r GitRunner) func() {
	prev := runner
	runner = r
	return func() { runner = prev }
}
