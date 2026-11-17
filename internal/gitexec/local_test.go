package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSwitchArgsOrder(t *testing.T) {
	got := switchArgs(SwitchRequest{
		NoGuess:     true,
		Discard:     true,
		Merge:       true,
		Create:      true,
		ForceCreate: true,
		Detach:      true,
		Opts:        []string{"--ignore-other-worktrees"},
		Target:      "feature",
		StartPoint:  "main",
	})
	want := "switch --no-guess --discard-changes -C feature --detach --ignore-other-worktrees -- main"
	if strings.Join(got, " ") != want {
		t.Fatalf("%v", got)
	}
	plain := switchArgs(SwitchRequest{Opts: []string{"--ff-only"}, Target: "main"})
	if strings.Join(plain, " ") != "switch --ff-only -- main" {
		t.Fatalf("%v", plain)
	}
	detached := switchArgs(SwitchRequest{Detach: true, Opts: []string{"-q"}, Target: "abc"})
	if strings.Join(detached, " ") != "switch --detach -q -- abc" {
		t.Fatalf("%v", detached)
	}
	created := switchArgs(SwitchRequest{NoGuess: true, Create: true, Target: "feature"})
	if strings.Join(created, " ") != "switch --no-guess -c feature" {
		t.Fatalf("%v", created)
	}
}

func TestLocalFakeGit(t *testing.T) {
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHERD_FAKE_GIT", "1")
	work := t.TempDir()
	ctx := context.Background()

	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "refs/heads/feature")
	branch, res, err := SymbolicHead(ctx, work)
	if err != nil || branch != "feature" || !strings.Contains(res.Stderr, "symbolic-ref --quiet HEAD") {
		t.Fatalf("branch=%q err=%v stderr=%q", branch, err, res.Stderr)
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "1")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "")
	branch, res, err = SymbolicHead(ctx, work)
	if err != nil || branch != "" || res.Exit != 1 {
		t.Fatalf("branch=%q exit=%d err=%v", branch, res.Exit, err)
	}

	ok, res, err := VerifyQuiet(ctx, work, "refs/heads/feature")
	if err != nil || ok || res.Exit != 1 {
		t.Fatalf("ok=%v exit=%d err=%v", ok, res.Exit, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "128")
	if _, _, err = VerifyQuiet(ctx, work, "refs/heads/feature"); err == nil {
		t.Fatal("expected verify error")
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "1")
	_, missing, res, err := HeadCommitQuiet(ctx, work)
	if err != nil || !missing || res.Exit != 1 {
		t.Fatalf("missing=%v exit=%d err=%v", missing, res.Exit, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "abc")
	oid, missing, _, err := HeadCommitQuiet(ctx, work)
	if err != nil || missing || oid != "abc" {
		t.Fatalf("oid=%q missing=%v err=%v", oid, missing, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_STDOUT", " M a.go\n?? b.go")
	dirty, _, err := Porcelain(ctx, work)
	if err != nil || !dirty {
		t.Fatalf("dirty=%v err=%v", dirty, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "\n")
	dirty, _, err = Porcelain(ctx, work)
	if err != nil || dirty {
		t.Fatalf("dirty=%v err=%v", dirty, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_STDOUT", ".git/rebase-merge")
	path, res, err := GitPath(ctx, work, "rebase-merge")
	if err != nil || path != ".git/rebase-merge" || !strings.Contains(res.Stderr, "--git-path rebase-merge") {
		t.Fatalf("path=%q err=%v stderr=%q", path, err, res.Stderr)
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "1")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "")
	value, res, err := ConfigGet(ctx, "init.defaultBranch")
	if err == nil || value != "" || res.Exit != 1 || !strings.Contains(res.Stderr, "config --get init.defaultBranch") {
		t.Fatalf("value=%q exit=%d err=%v stderr=%q", value, res.Exit, err, res.Stderr)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "develop")
	value, _, err = ConfigGet(ctx, "init.defaultBranch")
	if err != nil || value != "develop" {
		t.Fatalf("value=%q err=%v", value, err)
	}

	res, err = BranchUpstream(ctx, work, "feature")
	if err != nil || !strings.Contains(res.Stderr, "refs/heads/feature@{upstream}") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
	res, err = SetUpstream(ctx, work, "origin", "feature")
	if err != nil || !strings.Contains(res.Stderr, "branch --set-upstream-to=origin/feature -- feature") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
	res, err = Fetch(ctx, work, []string{"-c", "k=v"}, []string{"--prune"}, "origin")
	if err != nil || !strings.Contains(res.Stderr, "-c k=v fetch --prune -- origin") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
	res, err = Switch(ctx, work, SwitchRequest{NoGuess: true, Create: true, Opts: []string{"--quiet"}, Target: "feature", StartPoint: "HEAD"})
	if err != nil || !strings.Contains(res.Stderr, "switch --no-guess -c feature --quiet -- HEAD") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
	res, err = MergeBranch(ctx, work, []string{"--ff-only"}, "feature")
	if err != nil || !strings.Contains(res.Stderr, "merge --no-edit --ff-only -- feature") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
	res, err = DeleteBranch(ctx, work, []string{"-q"}, "feature")
	if err != nil || !strings.Contains(res.Stderr, "branch -d -q -- feature") {
		t.Fatalf("%q %v", res.Stderr, err)
	}
}
