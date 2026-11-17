package switchcmd

import (
	"context"
	"fmt"

	"github.com/git-herder/git-herder/internal/gitexec"
)

type fakeGit struct {
	verifyExit int
	verifyErr  error
	verified   []string

	symName  string
	symAfter string
	symErr   error
	symCalls int

	oid        string
	oidAfter   string
	oidMissing bool
	oidErr     error
	oidCalls   int

	dirty    bool
	dirtyErr error

	unmerged    []string
	unmergedErr error

	switchExit   int
	switchStderr string
	switches     int
	lastReq      gitexec.SwitchRequest

	upLine    string
	upExit    int
	upErr     error
	upCalls   int
	setExit   int
	setErr    error
	setStderr string
	sets      int
}

func (f *fakeGit) VerifyQuiet(_ context.Context, _ string, rev string) (bool, gitOut, error) {
	f.verified = append(f.verified, rev)
	if f.verifyErr != nil {
		return false, gitOut{}, f.verifyErr
	}
	if f.verifyExit == 1 {
		return false, gitOut{Exit: 1}, nil
	}
	if f.verifyExit != 0 {
		return false, gitOut{Exit: f.verifyExit}, fmt.Errorf("git rev-parse")
	}
	return true, gitOut{}, nil
}

func (f *fakeGit) SymbolicHead(context.Context, string) (string, gitOut, error) {
	f.symCalls++
	if f.symErr != nil {
		return "", gitOut{}, f.symErr
	}
	name := f.symName
	if f.symCalls > 1 {
		name = f.symAfter
	}
	return name, gitOut{Stdout: name}, nil
}

func (f *fakeGit) HeadCommit(context.Context, string) (string, bool, gitOut, error) {
	f.oidCalls++
	if f.oidErr != nil {
		return "", false, gitOut{}, f.oidErr
	}
	if f.oidCalls == 1 && f.oidMissing {
		return "", true, gitOut{Exit: 1}, nil
	}
	oid := f.oid
	if f.oidCalls > 1 && f.oidAfter != "" {
		oid = f.oidAfter
	}
	return oid, false, gitOut{Stdout: oid}, nil
}

func (f *fakeGit) Porcelain(context.Context, string) (bool, gitOut, error) {
	if f.dirtyErr != nil {
		return false, gitOut{}, f.dirtyErr
	}
	return f.dirty, gitOut{}, nil
}

func (f *fakeGit) Unmerged(context.Context, string) ([]string, gitOut, error) {
	if f.unmergedErr != nil {
		return nil, gitOut{Stderr: "diff failed"}, f.unmergedErr
	}
	return append([]string{}, f.unmerged...), gitOut{}, nil
}

func (f *fakeGit) Switch(_ context.Context, _ string, req gitexec.SwitchRequest) (gitOut, error) {
	f.switches++
	f.lastReq = req
	if f.switchExit != 0 {
		return gitOut{Exit: f.switchExit, Stderr: f.switchStderr}, fmt.Errorf("git switch")
	}
	return gitOut{}, nil
}

func (f *fakeGit) BranchUpstream(context.Context, string, string) (gitOut, error) {
	f.upCalls++
	if f.upErr != nil && f.upExit == 0 {
		return gitOut{}, f.upErr
	}
	if f.upExit != 0 {
		return gitOut{Exit: f.upExit}, fmt.Errorf("git rev-parse")
	}
	return gitOut{Stdout: f.upLine}, nil
}

func (f *fakeGit) SetUpstream(context.Context, string, string, string) (gitOut, error) {
	f.sets++
	if f.setErr != nil && f.setExit == 0 {
		return gitOut{}, f.setErr
	}
	if f.setExit != 0 {
		return gitOut{Exit: f.setExit, Stderr: f.setStderr}, fmt.Errorf("git branch")
	}
	return gitOut{}, nil
}
