package switchcmd

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
	Porcelain(ctx context.Context, dir string) (dirty bool, out gitOut, err error)
	Unmerged(ctx context.Context, dir string) (names []string, out gitOut, err error)
	Switch(ctx context.Context, dir string, req gitexec.SwitchRequest) (gitOut, error)
	BranchUpstream(ctx context.Context, dir, branch string) (gitOut, error)
	SetUpstream(ctx context.Context, dir, remote, branch string) (gitOut, error)
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

func (execGit) Porcelain(ctx context.Context, dir string) (bool, gitOut, error) {
	dirty, res, err := gitexec.Porcelain(ctx, dir)
	return dirty, fromResult(res), err
}

func (execGit) Unmerged(ctx context.Context, dir string) ([]string, gitOut, error) {
	names, res, err := gitexec.Unmerged(ctx, dir)
	return names, fromResult(res), err
}

func (execGit) Switch(ctx context.Context, dir string, req gitexec.SwitchRequest) (gitOut, error) {
	res, err := gitexec.Switch(ctx, dir, req)
	return fromResult(res), err
}

func (execGit) BranchUpstream(ctx context.Context, dir, branch string) (gitOut, error) {
	res, err := gitexec.BranchUpstream(ctx, dir, branch)
	return fromResult(res), err
}

func (execGit) SetUpstream(ctx context.Context, dir, remote, branch string) (gitOut, error) {
	res, err := gitexec.SetUpstream(ctx, dir, remote, branch)
	return fromResult(res), err
}

func fromResult(res gitexec.Result) gitOut {
	return gitOut{Stdout: res.Stdout, Stderr: res.Stderr, Exit: res.Exit}
}

var runner gitOps = execGit{}
