package sync

import (
	"context"

	"github.com/git-herder/git-herder/internal/gitexec"
)

// gitOut — раздельные потоки одного запуска git.
type gitOut struct {
	Stdout string
	Stderr string
	Exit   int
}

// gitOps — операции git в каталоге рабочей копии.
type gitOps interface {
	Head(ctx context.Context, dir string) (branch string, detached bool, output string, err error)
	RemoteNames(ctx context.Context, dir string) (names []string, out gitOut, err error)
	RemoteURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error)
	LsRemoteHead(ctx context.Context, dir, remote, branch string, cfg []string) (oid string, found bool, out gitOut, err error)
	HeadOID(ctx context.Context, dir string) (oid string, out gitOut, err error)
	Pull(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error)
	Push(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error)
	MergeHead(ctx context.Context, dir string) (ok bool, out gitOut, err error)
	Unmerged(ctx context.Context, dir string) (names []string, out gitOut, err error)
}

type execGit struct{}

func (execGit) Head(ctx context.Context, dir string) (string, bool, string, error) {
	return gitexec.HeadBranch(ctx, dir)
}

func (execGit) RemoteNames(ctx context.Context, dir string) ([]string, gitOut, error) {
	names, res, err := gitexec.RemoteNames(ctx, dir)
	return names, fromResult(res), err
}

func (execGit) RemoteURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	return gitexec.RemoteURL(ctx, dir, name)
}

func (execGit) LsRemoteHead(ctx context.Context, dir, remote, branch string, cfg []string) (string, bool, gitOut, error) {
	oid, found, res, err := gitexec.LsRemoteHead(ctx, dir, remote, branch, cfg)
	return oid, found, fromResult(res), err
}

func (execGit) HeadOID(ctx context.Context, dir string) (string, gitOut, error) {
	oid, res, err := gitexec.HeadOID(ctx, dir)
	return oid, fromResult(res), err
}

func (execGit) Pull(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error) {
	res, err := gitexec.Pull(ctx, dir, opts, remote, branch, cfg)
	return fromResult(res), err
}

func (execGit) Push(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error) {
	res, err := gitexec.PushOpts(ctx, dir, opts, remote, branch, cfg)
	return fromResult(res), err
}

func (execGit) MergeHead(ctx context.Context, dir string) (bool, gitOut, error) {
	ok, res, err := gitexec.MergeHead(ctx, dir)
	return ok, fromResult(res), err
}

func (execGit) Unmerged(ctx context.Context, dir string) ([]string, gitOut, error) {
	names, res, err := gitexec.Unmerged(ctx, dir)
	return names, fromResult(res), err
}

func fromResult(res gitexec.Result) gitOut {
	return gitOut{Stdout: res.Stdout, Stderr: res.Stderr, Exit: res.Exit}
}

var runner gitOps = execGit{}
