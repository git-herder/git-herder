// Исполнение критериев отбора через GitCliRunner.
package workspace

import (
	"context"

	"github.com/git-herder/git-herder/internal/gitexec"
)

// GitExecutor — вызовы git, которыми резолвер проверяет критерии.
// Совпадает с функциями пакета gitexec, которые нужны критериям.
type GitExecutor interface {
	VerifyQuiet(ctx context.Context, dir, rev string) (ok bool, res gitexec.Result, err error)
	SymbolicHead(ctx context.Context, dir string) (name string, res gitexec.Result, err error)
	Remotes(ctx context.Context, dir string) (urls []string, output string, err error)
}

// execGit — боевой GitCliRunner: git из PATH машины пользователя.
type execGit struct{}

func (execGit) VerifyQuiet(ctx context.Context, dir, rev string) (bool, gitexec.Result, error) {
	return gitexec.VerifyQuiet(ctx, dir, rev)
}

func (execGit) SymbolicHead(ctx context.Context, dir string) (string, gitexec.Result, error) {
	return gitexec.SymbolicHead(ctx, dir)
}

func (execGit) Remotes(ctx context.Context, dir string) ([]string, string, error) {
	return gitexec.Remotes(ctx, dir)
}

// git — исполнение критериев. В тесте подменяется InstallGitForTest.
var git GitExecutor = execGit{}

// InstallGitForTest подменяет исполнение git критериев и возвращает восстановление.
func InstallGitForTest(g GitExecutor) func() {
	prev := git
	if g != nil {
		git = g
	}
	return func() { git = prev }
}
