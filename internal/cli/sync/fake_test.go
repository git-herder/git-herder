package sync

import (
	"context"
	"fmt"
)

type fakeGit struct {
	branch       string
	detached     bool
	headErr      error
	names        []string
	namesErr     error
	remoteURL    string
	urlMissing   bool
	urlErr       error
	lsOID        string
	lsFound      bool
	lsExit       int
	lsStderr     string
	lsSeq        []int
	failDir      string
	oid          string
	oidAfterPull string
	oidErrOn     int
	pullExit     int
	pullStderr   string
	pullStdout   string
	merge        bool
	mergeErr     error
	unmerged     []string
	unmergedErr  error
	pushExit     int
	pushStderr   string
	heads        int
	lses         int
	pulls        int
	pushes       int
	oidCalls     int
	lastCfg      []string
	lastPullOpts []string
	lastPushOpts []string
	lastRemote   string
	lastBranch   string
	calls        []string
	cfgs         [][]string
}

func (f *fakeGit) Head(ctx context.Context, dir string) (string, bool, string, error) {
	f.heads++
	f.calls = append(f.calls, "head:"+dir)
	if f.headErr != nil {
		return "", false, "", f.headErr
	}
	return f.branch, f.detached, f.branch, nil
}

func (f *fakeGit) RemoteNames(ctx context.Context, dir string) ([]string, gitOut, error) {
	f.calls = append(f.calls, "names:"+dir)
	if f.namesErr != nil {
		return nil, gitOut{Stderr: "names failed"}, f.namesErr
	}
	return append([]string{}, f.names...), gitOut{}, nil
}

func (f *fakeGit) RemoteURL(ctx context.Context, dir, name string) (string, bool, string, error) {
	f.calls = append(f.calls, "url:"+name)
	if f.urlErr != nil {
		return "", false, "", f.urlErr
	}
	return f.remoteURL, f.urlMissing, f.remoteURL, nil
}

func (f *fakeGit) LsRemoteHead(ctx context.Context, dir, remote, branch string, cfg []string) (string, bool, gitOut, error) {
	f.lses++
	f.calls = append(f.calls, "ls:"+dir)
	f.noteCall(branch, cfg)
	f.lastRemote = remote
	exit := f.lsExit
	if len(f.lsSeq) > 0 {
		exit = f.lsSeq[0]
		f.lsSeq = f.lsSeq[1:]
	}
	if f.failDir != "" && dir == f.failDir {
		exit = 128
	}
	out := gitOut{Exit: exit, Stderr: f.lsStderr}
	if exit == 128 {
		if out.Stderr == "" {
			out.Stderr = "down"
		}
		return "", false, out, fmt.Errorf("ls-remote failed")
	}
	if exit != 0 {
		return "", false, out, fmt.Errorf("ls-remote exit %d", exit)
	}
	if !f.lsFound {
		return "", false, out, nil
	}
	return f.lsOID, true, out, nil
}

func (f *fakeGit) HeadOID(ctx context.Context, dir string) (string, gitOut, error) {
	f.oidCalls++
	if f.oidErrOn == f.oidCalls {
		return "", gitOut{Stderr: "no head"}, fmt.Errorf("no head")
	}
	oid := f.oid
	if f.pulls > 0 && f.oidAfterPull != "" {
		oid = f.oidAfterPull
	}
	return oid, gitOut{Stdout: oid}, nil
}

func (f *fakeGit) Pull(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error) {
	f.pulls++
	f.lastPullOpts = append([]string{}, opts...)
	f.noteCall(branch, cfg)
	f.lastRemote = remote
	out := gitOut{Exit: f.pullExit, Stderr: f.pullStderr, Stdout: f.pullStdout}
	if f.pullExit != 0 {
		return out, fmt.Errorf("pull exit %d", f.pullExit)
	}
	return out, nil
}

func (f *fakeGit) Push(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (gitOut, error) {
	f.pushes++
	f.lastPushOpts = append([]string{}, opts...)
	f.noteCall(branch, cfg)
	f.lastRemote = remote
	out := gitOut{Exit: f.pushExit, Stderr: f.pushStderr}
	if f.pushExit != 0 {
		return out, fmt.Errorf("push exit %d", f.pushExit)
	}
	return out, nil
}

func (f *fakeGit) MergeHead(ctx context.Context, dir string) (bool, gitOut, error) {
	if f.mergeErr != nil {
		return false, gitOut{}, f.mergeErr
	}
	return f.merge, gitOut{}, nil
}

func (f *fakeGit) Unmerged(ctx context.Context, dir string) ([]string, gitOut, error) {
	if f.unmergedErr != nil {
		return nil, gitOut{Stderr: "diff failed"}, f.unmergedErr
	}
	return append([]string{}, f.unmerged...), gitOut{}, nil
}

func (f *fakeGit) noteCall(branch string, cfg []string) {
	f.lastBranch = branch
	f.lastCfg = append([]string{}, cfg...)
	f.cfgs = append(f.cfgs, append([]string{}, cfg...))
}

func readyGit() *fakeGit {
	return &fakeGit{
		branch:  "feature",
		names:   []string{"origin"},
		lsFound: true,
		lsOID:   "remote",
		oid:     "local",
	}
}
