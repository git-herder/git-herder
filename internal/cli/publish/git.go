package publish

import (
	"context"

	"github.com/git-herder/git-herder/internal/gitexec"
)

// gitOps — операции git в каталоге рабочей копии.
type gitOps interface {
	Remotes(ctx context.Context, dir string) (urls []string, output string, err error)
	Head(ctx context.Context, dir string) (branch string, detached bool, output string, err error)
	HasBranch(ctx context.Context, dir, name string) (ok bool, output string, err error)
	RemoteURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error)
	RemoteAdd(ctx context.Context, dir, name, remoteURL string) (output string, err error)
	RemoteRename(ctx context.Context, dir, oldName, newName string) (output string, err error)
	Push(ctx context.Context, dir, remote, branch string) (output string, err error)
}

type execGit struct{}

func (execGit) Remotes(ctx context.Context, dir string) ([]string, string, error) {
	return gitexec.Remotes(ctx, dir)
}

func (execGit) Head(ctx context.Context, dir string) (string, bool, string, error) {
	return gitexec.HeadBranch(ctx, dir)
}

func (execGit) HasBranch(ctx context.Context, dir, name string) (bool, string, error) {
	return gitexec.LocalBranch(ctx, dir, name)
}

func (execGit) RemoteURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	return gitexec.RemoteURL(ctx, dir, name)
}

func (execGit) RemoteAdd(ctx context.Context, dir, name, remoteURL string) (string, error) {
	return gitexec.RemoteAdd(ctx, dir, name, remoteURL)
}

func (execGit) RemoteRename(ctx context.Context, dir, oldName, newName string) (string, error) {
	return gitexec.RemoteRename(ctx, dir, oldName, newName)
}

func (execGit) Push(ctx context.Context, dir, remote, branch string) (string, error) {
	return gitexec.Push(ctx, dir, remote, branch)
}

var runner gitOps = execGit{}
