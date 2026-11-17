package switchcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/workspace"
)

func install(t *testing.T, git gitOps) *[]string {
	t.Helper()
	prevR, prevP, prevC, prevPick := runner, printLine, confirmMass, asPicker
	var lines []string
	runner = git
	printLine = func(s string) { lines = append(lines, s) }
	t.Cleanup(func() {
		runner = prevR
		printLine = prevP
		confirmMass = prevC
		asPicker = prevPick
	})
	return &lines
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

func TestSwitchValidation(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	ws := t.TempDir()
	cases := [][]string{
		{"--workspace", ws, "switch"},
		{"--workspace", ws, "switch", "feature", "--interactive", "--all"},
		{"--workspace", ws, "switch", "feature", "HEAD"},
		{"--workspace", ws, "switch", "feature", "", "--create"},
		{"--workspace", ws, "switch", "feature", "--create", "--force-create"},
		{"--workspace", ws, "switch", "feature", "--discard-changes", "--merge"},
		{"--workspace", ws, "switch", "feature", "--track="},
		{"--workspace", ws, "switch", "feature", "--git-opts=--detach"},
	}
	for _, args := range cases {
		if err := runCmd(t, args...); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	file := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "--workspace", file, "switch", "feature"); err == nil {
		t.Fatal("expected workspace error")
	}
}

func TestSwitchFilterAndProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	body := "root:\n  branch: from-root\nswitch:\n  branch: feature\n"
	if err := os.WriteFile(filepath.Join(dir, "ok.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{symName: "feature", symAfter: "feature", oid: "aaa"}
	lines := install(t, git)
	ws := t.TempDir()
	if err := runCmd(t, "--profile", "ok", "--workspace", ws, "switch"); err != nil {
		t.Fatal(err)
	}
	if git.switches != 0 || len(git.verified) != 0 {
		t.Fatalf("empty set switches %d verifies %v", git.switches, git.verified)
	}
	makeCopy(t, ws, "documentation/architecture")
	git.verifyExit = 1
	if err := runCmd(t, "--profile", "ok", "--workspace", ws, "switch"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 0 || git.switches != 0 || len(git.verified) != 1 {
		t.Fatalf("lines %v switches %d verifies %v", *lines, git.switches, git.verified)
	}
	git.verifyExit = 128
	if err := runCmd(t, "--workspace", ws, "switch", "feature"); err == nil {
		t.Fatal("expected filter error")
	}
	git.verifyExit = 0
	git.verified = nil
	if err := runCmd(t, "--workspace", ws, "switch", "feature", "--create"); err != nil {
		t.Fatal(err)
	}
	if len(git.verified) != 0 || git.switches != 1 {
		t.Fatalf("create verifies %v switches %d", git.verified, git.switches)
	}
	if len(*lines) != 1 || (*lines)[0] != "➡️ ➡️ documentation/architecture" {
		t.Fatalf("%v", *lines)
	}
}

func TestSwitchAbortAndRetries(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{symName: "main", oid: "aaa", switchExit: 1, switchStderr: "switch failed"}
	lines := install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "a")
	makeCopy(t, ws, "b")
	err := runCmd(t, "--retries-number", "5", "--on-error", "abort", "--workspace", ws, "switch", "feature")
	if err == nil {
		t.Fatal("expected failure")
	}
	if git.switches != 1 || len(*lines) != 1 {
		t.Fatalf("switches %d lines %v", git.switches, *lines)
	}
	git.switches = 0
	*lines = nil
	err = runCmd(t, "--retries-number", "5", "--on-error", "continue", "--workspace", ws, "switch", "feature")
	if err == nil || git.switches != 2 || len(*lines) != 2 {
		t.Fatalf("err %v switches %d lines %d", err, git.switches, len(*lines))
	}
}

func TestSwitchMassConfirm(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{symName: "feature", symAfter: "feature", oid: "aaa"}
	install(t, git)
	many := t.TempDir()
	for i := 0; i < 11; i++ {
		makeCopy(t, many, filepath.Join("n", string(rune('a'+i))))
	}
	prev := confirmMass
	confirmMass = func(*console.UI, int) (bool, error) { return false, nil }
	t.Cleanup(func() { confirmMass = prev })
	if err := runCmd(t, "--workspace", many, "switch", "feature"); err != nil {
		t.Fatal(err)
	}
	if git.switches != 0 {
		t.Fatalf("declined switches %d", git.switches)
	}
	called := false
	confirmMass = func(*console.UI, int) (bool, error) {
		called = true
		return false, nil
	}
	if err := runCmd(t, "--workspace", many, "switch", "feature", "--all"); err != nil {
		t.Fatal(err)
	}
	if called || git.switches != 11 {
		t.Fatalf("called %v switches %d", called, git.switches)
	}
}

func TestSwitchInteractiveCancel(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{}
	install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	asPicker = func(*console.UI) workspace.Picker { return cancelPicker{} }
	if err := runCmd(t, "--workspace", ws, "switch", "feature", "--interactive"); err != nil {
		t.Fatal(err)
	}
	if git.switches != 0 {
		t.Fatal(git.switches)
	}
}

type cancelPicker struct{}

func (cancelPicker) Select([]console.SelectItem) ([]int, error) {
	return nil, console.ErrSelectCancelled
}
