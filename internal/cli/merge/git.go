package merge

import (
	"context"

	"github.com/git-herder/git-herder/internal/gitexec"
)

type gitOut struct {
	Stdout string
	Stderr string
	Exit   int
}

type gitOps interface {
	VerifyQuiet(ctx context.Context, dir, rev string) (ok bool, out gitOut, err error)
	SymbolicHead(ctx context.Context, dir string) (name string, out gitOut, err error)
	HeadCommit(ctx context.Context, dir string) (oid string, missing bool, out gitOut, err error)
	HeadVerify(ctx context.Context, dir string) (oid string, out gitOut, err error)
	Porcelain(ctx context.Context, dir string) (dirty bool, out gitOut, err error)
	Unmerged(ctx context.Context, dir string) (names []string, out gitOut, err error)
	MergeHead(ctx context.Context, dir string) (ok bool, out gitOut, err error)
	GitPath(ctx context.Context, dir, name string) (path string, out gitOut, err error)
	RemoteURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error)
	Fetch(ctx context.Context, dir string, cfg, opts []string, remote string) (gitOut, error)
	Switch(ctx context.Context, dir string, req gitexec.SwitchRequest) (gitOut, error)
	MergeBranch(ctx context.Context, dir string, opts []string, from string) (gitOut, error)
	DeleteBranch(ctx context.Context, dir string, opts []string, name string) (gitOut, error)
	DefaultBranch(ctx context.Context) (name string, out gitOut, err error)
}

type execGit struct{}

func (execGit) VerifyQuiet(ctx context.Context, dir, rev string) (bool, gitOut, error) {
	ok, res, err := gitexec.VerifyQuiet(ctx, dir, rev)
	return ok, fromResult(res), err
}

func (execGit) SymbolicHead(ctx context.Context, dir string) (string, gitOut, error) {
	name, res, err := gitexec.SymbolicHead(ctx, dir)
	return name, fromResult(res), err
}

func (execGit) HeadCommit(ctx context.Context, dir string) (string, bool, gitOut, error) {
	oid, missing, res, err := gitexec.HeadCommitQuiet(ctx, dir)
	return oid, missing, fromResult(res), err
}

func (execGit) HeadVerify(ctx context.Context, dir string) (string, gitOut, error) {
	oid, res, err := gitexec.HeadCommitVerify(ctx, dir)
	return oid, fromResult(res), err
}

func (execGit) Porcelain(ctx context.Context, dir string) (bool, gitOut, error) {
	dirty, res, err := gitexec.Porcelain(ctx, dir)
	return dirty, fromResult(res), err
}

func (execGit) Unmerged(ctx context.Context, dir string) ([]string, gitOut, error) {
	names, res, err := gitexec.Unmerged(ctx, dir)
	return names, fromResult(res), err
}

func (execGit) MergeHead(ctx context.Context, dir string) (bool, gitOut, error) {
	ok, res, err := gitexec.MergeHead(ctx, dir)
	return ok, fromResult(res), err
}

func (execGit) GitPath(ctx context.Context, dir, name string) (string, gitOut, error) {
	path, res, err := gitexec.GitPath(ctx, dir, name)
	return path, fromResult(res), err
}

func (execGit) RemoteURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	return gitexec.RemoteURL(ctx, dir, name)
}

func (execGit) Fetch(ctx context.Context, dir string, cfg, opts []string, remote string) (gitOut, error) {
	res, err := gitexec.Fetch(ctx, dir, cfg, opts, remote)
	return fromResult(res), err
}

func (execGit) Switch(ctx context.Context, dir string, req gitexec.SwitchRequest) (gitOut, error) {
	res, err := gitexec.Switch(ctx, dir, req)
	return fromResult(res), err
}

func (execGit) MergeBranch(ctx context.Context, dir string, opts []string, from string) (gitOut, error) {
	res, err := gitexec.MergeBranch(ctx, dir, opts, from)
	return fromResult(res), err
}

func (execGit) DeleteBranch(ctx context.Context, dir string, opts []string, name string) (gitOut, error) {
	res, err := gitexec.DeleteBranch(ctx, dir, opts, name)
	return fromResult(res), err
}

func (execGit) DefaultBranch(ctx context.Context) (string, gitOut, error) {
	name, res, err := gitexec.ConfigGet(ctx, "init.defaultBranch")
	return name, fromResult(res), err
}

func fromResult(res gitexec.Result) gitOut {
	return gitOut{Stdout: res.Stdout, Stderr: res.Stderr, Exit: res.Exit}
}

var runner gitOps = execGit{}
