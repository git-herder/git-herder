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
	RemoteNames(ctx context.Context, dir string) (names []string, res gitexec.Result, err error)
	RemoteURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error)
	RemotePushURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error)
}

// execGit — боевой GitCliRunner: git из PATH машины пользователя.
type execGit struct{}

func (execGit) VerifyQuiet(ctx context.Context, dir, rev string) (bool, gitexec.Result, error) {
	return gitexec.VerifyQuiet(ctx, dir, rev)
}

func (execGit) SymbolicHead(ctx context.Context, dir string) (string, gitexec.Result, error) {
	return gitexec.SymbolicHead(ctx, dir)
}

func (execGit) RemoteNames(ctx context.Context, dir string) ([]string, gitexec.Result, error) {
	return gitexec.RemoteNames(ctx, dir)
}

func (execGit) RemoteURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	return gitexec.RemoteURL(ctx, dir, name)
}

func (execGit) RemotePushURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	return gitexec.RemotePushURL(ctx, dir, name)
}
