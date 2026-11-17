package merge

import (
	"context"
	"fmt"

	"github.com/git-herder/git-herder/internal/gitexec"
)

type fakeGit struct {
	missingFrom bool
	verifyErr   error
	verified    []string
	inProgress  bool
	mergeHead   bool
	cherry      bool

	entryBranch   string
	entryDetach   bool
	entryOID      string
	entryErr      error
	headVerifyErr error
	afterBranch   string
	afterSymErr   error
	symCalls      int

	oidAfterSwitch string
	oidAfterMerge  string
	oidErr         error
	oidCalls       int

	dirty      bool
	dirtyErr   error
	rebase     string
	gitPathErr error

	unmerged    []string
	unmergedErr error

	remoteURL  string
	urlMissing bool
	urlErr     error

	fetchExit   int
	fetchStderr string
	fetches     int
	lastCfg     []string
	lastFetch   []string
	lastRemote  string

	switchExit   int
	switchStderr string
	switchReqs   []gitexec.SwitchRequest
	returnExit   int
	returnStderr string

	mergeExit   int
	mergeStderr string
	merges      int
	lastMerge   []string

	deleteExit   int
	deleteStderr string
	deletes      int
	lastDelete   []string

	defaultName string
	defaultExit int
	defaultErr  error
	defaults    int
}

func (f *fakeGit) VerifyQuiet(_ context.Context, _ string, rev string) (bool, gitOut, error) {
	f.verified = append(f.verified, rev)
	if f.verifyErr != nil {
		return false, gitOut{}, f.verifyErr
	}
	if rev == "MERGE_HEAD" && f.inProgress {
		return true, gitOut{}, nil
	}
	if rev == "CHERRY_PICK_HEAD" && f.cherry {
		return true, gitOut{}, nil
	}
	if len(rev) > len("refs/heads/") && rev[:len("refs/heads/")] == "refs/heads/" {
		if f.missingFrom {
			return false, gitOut{Exit: 1}, nil
		}
		return true, gitOut{}, nil
	}
	return false, gitOut{Exit: 1}, nil
}

func (f *fakeGit) SymbolicHead(context.Context, string) (string, gitOut, error) {
	f.symCalls++
	if f.symCalls == 1 {
		if f.entryErr != nil {
			return "", gitOut{}, f.entryErr
		}
		if f.entryDetach {
			return "", gitOut{Exit: 1}, nil
		}
		return f.entryBranch, gitOut{Stdout: f.entryBranch}, nil
	}
	if f.afterSymErr != nil {
		return "", gitOut{}, f.afterSymErr
	}
	return f.afterBranch, gitOut{Stdout: f.afterBranch}, nil
}

func (f *fakeGit) HeadVerify(context.Context, string) (string, gitOut, error) {
	if f.headVerifyErr != nil {
		return "", gitOut{}, f.headVerifyErr
	}
	if f.entryOID == "" {
		return "", gitOut{Exit: 1}, fmt.Errorf("git rev-parse")
	}
	return f.entryOID, gitOut{Stdout: f.entryOID}, nil
}

func (f *fakeGit) HeadCommit(context.Context, string) (string, bool, gitOut, error) {
	f.oidCalls++
	if f.oidErr != nil {
		return "", false, gitOut{}, f.oidErr
	}
	oid := f.oidAfterSwitch
	if f.oidCalls > 1 {
		oid = f.oidAfterMerge
		if oid == "" {
			oid = f.oidAfterSwitch
		}
	}
	if oid == "" {
		return "", true, gitOut{Exit: 1}, nil
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

func (f *fakeGit) MergeHead(context.Context, string) (bool, gitOut, error) {
	return f.mergeHead, gitOut{}, nil
}

func (f *fakeGit) GitPath(context.Context, string, string) (string, gitOut, error) {
	if f.gitPathErr != nil {
		return "", gitOut{}, f.gitPathErr
	}
	if f.rebase != "" {
		return f.rebase, gitOut{Stdout: f.rebase}, nil
	}
	return "missing-rebase", gitOut{Stdout: "missing-rebase"}, nil
}

func (f *fakeGit) RemoteURL(context.Context, string, string) (string, bool, string, error) {
	if f.urlErr != nil {
		return "", false, "", f.urlErr
	}
	if f.urlMissing || f.remoteURL == "" {
		return "", true, "", nil
	}
	return f.remoteURL, false, f.remoteURL, nil
}

func (f *fakeGit) Fetch(_ context.Context, _ string, cfg, opts []string, remote string) (gitOut, error) {
	f.fetches++
	f.lastCfg = append([]string{}, cfg...)
	f.lastFetch = append([]string{}, opts...)
	f.lastRemote = remote
	if f.fetchExit != 0 {
		return gitOut{Exit: f.fetchExit, Stderr: f.fetchStderr}, fmt.Errorf("git fetch")
	}
	return gitOut{}, nil
}

func (f *fakeGit) Switch(_ context.Context, _ string, req gitexec.SwitchRequest) (gitOut, error) {
	f.switchReqs = append(f.switchReqs, req)
	if len(f.switchReqs) == 1 && f.switchExit != 0 {
		return gitOut{Exit: f.switchExit, Stderr: f.switchStderr}, fmt.Errorf("git switch")
	}
	if len(f.switchReqs) > 1 && f.returnExit != 0 {
		return gitOut{Exit: f.returnExit, Stderr: f.returnStderr}, fmt.Errorf("git switch")
	}
	return gitOut{}, nil
}

func (f *fakeGit) MergeBranch(_ context.Context, _ string, opts []string, _ string) (gitOut, error) {
	f.merges++
	f.lastMerge = append([]string{}, opts...)
	if f.mergeExit != 0 {
		return gitOut{Exit: f.mergeExit, Stderr: f.mergeStderr}, fmt.Errorf("git merge")
	}
	return gitOut{}, nil
}

func (f *fakeGit) DeleteBranch(_ context.Context, _ string, opts []string, _ string) (gitOut, error) {
	f.deletes++
	f.lastDelete = append([]string{}, opts...)
	if f.deleteExit != 0 {
		return gitOut{Exit: f.deleteExit, Stderr: f.deleteStderr}, fmt.Errorf("git branch")
	}
	return gitOut{}, nil
}

func (f *fakeGit) DefaultBranch(context.Context) (string, gitOut, error) {
	f.defaults++
	if f.defaultErr != nil && f.defaultExit == 0 {
		return "", gitOut{}, f.defaultErr
	}
	if f.defaultExit != 0 {
		return "", gitOut{Exit: f.defaultExit}, fmt.Errorf("git config")
	}
	return f.defaultName, gitOut{Stdout: f.defaultName}, nil
}
