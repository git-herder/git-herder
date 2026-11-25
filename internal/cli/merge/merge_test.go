package merge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/workspace"
)

func install(t *testing.T, git gitOps) *[]string {
	t.Helper()
	prevR, prevP, prevC, prevPick := runner, printLine, confirmMass, asPicker
	var lines []string
	runner = git
	printLine = func(s string) { lines = append(lines, s) }
	prevResolve := newResolver
	newResolver = func() *workspace.Resolver {
		return workspace.Resolve().Git(mergeWorkspaceGit{git: git})
	}
	t.Cleanup(func() {
		newResolver = prevResolve
		runner = prevR
		printLine = prevP
		confirmMass = prevC
		asPicker = prevPick
	})
	return &lines
}

// mergeWorkspaceGit подставляет git-критерий HasBranch: ответы берутся из fakeGit.
type mergeWorkspaceGit struct {
	git gitOps
}

func (m mergeWorkspaceGit) VerifyQuiet(ctx context.Context, dir, rev string) (bool, gitexec.Result, error) {
	f, ok := m.git.(*fakeGit)
	if !ok {
		return false, gitexec.Result{}, nil
	}
	ok2, out, err := f.VerifyQuiet(ctx, dir, rev)
	return ok2, gitexec.Result{Exit: out.Exit}, err
}

func (mergeWorkspaceGit) SymbolicHead(context.Context, string) (string, gitexec.Result, error) {
	return "", gitexec.Result{}, nil
}

func (mergeWorkspaceGit) RemoteNames(context.Context, string) ([]string, gitexec.Result, error) {
	return nil, gitexec.Result{}, nil
}

func (mergeWorkspaceGit) RemoteURL(context.Context, string, string) (string, bool, string, error) {
	return "", true, "", nil
}

func (mergeWorkspaceGit) RemotePushURL(context.Context, string, string) (string, bool, string, error) {
	return "", true, "", nil
}

func makeCopy(t *testing.T, root, rel string) {
	t.Helper()
	dir := root
	if rel != "" && rel != "." {
		dir = filepath.Join(root, filepath.FromSlash(rel))
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runCmd(t *testing.T, args ...string) error {
	t.Helper()
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs(args)
	root.SilenceErrors = true
	return root.Execute()
}

func TestMergeValidation(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	ws := t.TempDir()
	cases := [][]string{
		{"--workspace", ws, "merge"},
		{"--workspace", ws, "merge", "feature", "--interactive", "--all"},
		{"--workspace", ws, "merge", "feature", "main", "--remote="},
		{"--workspace", ws, "merge", "feature", ""},
		{"--workspace", ws, "merge", "feature", "feature"},
		{"--workspace", ws, "merge", "feature", "main", "--git-merge-opts=--abort"},
	}
	for _, args := range cases {
		if err := runCmd(t, args...); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestMergeDefaultBranchFilterAndPrecheck(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{defaultExit: 1, entryBranch: "other", afterBranch: "master", oidAfterSwitch: "aaa", oidAfterMerge: "bbb"}
	lines := install(t, git)
	ws := t.TempDir()
	if err := runCmd(t, "--workspace", ws, "merge", "feature"); err != nil {
		t.Fatal(err)
	}
	if git.defaults != 1 || git.fetches != 0 {
		t.Fatalf("defaults %d fetches %d", git.defaults, git.fetches)
	}
	makeCopy(t, ws, "documentation/architecture")
	git.missingFrom = true
	if err := runCmd(t, "--workspace", ws, "merge", "feature", "main"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 0 || git.fetches != 0 {
		t.Fatalf("lines %v fetches %d", *lines, git.fetches)
	}
	git.missingFrom = false
	git.dirty = true
	err := runCmd(t, "--workspace", ws, "merge", "feature", "main")
	if err == nil || git.fetches != 0 || len(*lines) != 1 {
		t.Fatalf("err %v fetches %d lines %v", err, git.fetches, *lines)
	}
	if (*lines)[0] != "⏩ ⏩ ⏩ ⏩ ⏩ documentation/architecture\n    uncommitted changes" {
		t.Fatalf("%q", (*lines)[0])
	}
	git.dirty = false
	git.defaults = 0
	*lines = nil
	if err := runCmd(t, "--workspace", ws, "merge", "feature"); err != nil {
		t.Fatal(err)
	}
	if git.defaults != 1 || len(*lines) != 1 || git.switchReqs[len(git.switchReqs)-1].Target != "master" {
		t.Fatalf("defaults %d lines %v reqs %+v", git.defaults, *lines, git.switchReqs)
	}
}

func TestMergeSameAsDefaultAndAbort(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{defaultName: "feature"}
	install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "a")
	if err := runCmd(t, "--workspace", ws, "merge", "feature"); err == nil || len(git.verified) != 0 {
		t.Fatalf("err nil verifies %v", git.verified)
	}
	git = &fakeGit{entryBranch: "main", fetchExit: 1, fetchStderr: "network"}
	lines := install(t, git)
	makeCopy(t, ws, "b")
	err := runCmd(t, "--retries-number", "4", "--on-error", "abort", "--workspace", ws, "merge", "feature", "main")
	if err == nil || git.fetches != 1 || len(*lines) != 1 {
		t.Fatalf("err %v fetches %d lines %d", err, git.fetches, len(*lines))
	}
}

func TestMergeMassConfirm(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{entryBranch: "main", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "aaa"}
	install(t, git)
	many := t.TempDir()
	for i := 0; i < 11; i++ {
		makeCopy(t, many, filepath.Join("n", string(rune('a'+i))))
	}
	prev := confirmMass
	confirmMass = func(*console.UI, int) (bool, error) { return false, nil }
	t.Cleanup(func() { confirmMass = prev })
	if err := runCmd(t, "--workspace", many, "merge", "feature", "main"); err != nil {
		t.Fatal(err)
	}
	if git.fetches != 0 {
		t.Fatalf("declined fetches %d", git.fetches)
	}
	if err := runCmd(t, "--workspace", many, "merge", "feature", "main", "--all"); err != nil {
		t.Fatal(err)
	}
	if git.fetches != 11 {
		t.Fatalf("fetches %d", git.fetches)
	}
}

func TestMergeInteractiveCancel(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{}
	install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	asPicker = func(*console.UI) workspace.Picker { return cancelPicker{} }
	if err := runCmd(t, "--workspace", ws, "merge", "feature", "main", "--interactive"); err != nil {
		t.Fatal(err)
	}
	if git.fetches != 0 {
		t.Fatal(git.fetches)
	}
}

type cancelPicker struct{}

func (cancelPicker) Select([]console.SelectItem) ([]int, error) {
	return nil, console.ErrSelectCancelled
}
