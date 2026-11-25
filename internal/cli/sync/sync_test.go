package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	restore := workspace.InstallGitForTest(syncWorkspaceGit{git: git})
	t.Cleanup(func() {
		restore()
		runner = prevR
		printLine = prevP
		confirmMass = prevC
		asPicker = prevPick
	})
	return &lines
}

// syncWorkspaceGit подставляет git-критерий OnBranch: ответы берутся из fakeGit.
type syncWorkspaceGit struct {
	git gitOps
}

func (s syncWorkspaceGit) VerifyQuiet(context.Context, string, string) (bool, gitexec.Result, error) {
	return false, gitexec.Result{}, nil
}

func (s syncWorkspaceGit) SymbolicHead(ctx context.Context, dir string) (string, gitexec.Result, error) {
	f, ok := s.git.(*fakeGit)
	if !ok {
		return "", gitexec.Result{}, nil
	}
	branch, detached, _, err := f.Head(ctx, dir)
	if err != nil {
		return "", gitexec.Result{}, err
	}
	if detached {
		return "", gitexec.Result{Exit: 1}, nil
	}
	return branch, gitexec.Result{}, nil
}

func (s syncWorkspaceGit) Remotes(context.Context, string) ([]string, string, error) {
	return nil, "", nil
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

func TestSyncRejectsBeforeSearch(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := readyGit()
	install(t, git)
	ws := t.TempDir()
	if err := runCmd(t, "--workspace", ws, "sync"); err == nil || !strings.Contains(err.Error(), "branch is required") {
		t.Fatal(err)
	}
	if err := runCmd(t, "--workspace", ws, "sync", "feature", "--interactive", "--all"); err == nil {
		t.Fatal("expected interactive and all error")
	}
	if err := runCmd(t, "--workspace", ws, "sync", "feature", "--remote="); err == nil || !strings.Contains(err.Error(), "remote name is empty") {
		t.Fatal(err)
	}
	if err := runCmd(t, "--workspace", ws, "sync", "feature", "--git-pull-opts=--rebase", "--git-push-opts=--force"); err == nil {
		t.Fatal("expected forbidden opts")
	}
	if git.heads != 0 || git.lses != 0 {
		t.Fatalf("searched anyway heads=%d lses=%d", git.heads, git.lses)
	}
	file := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "--workspace", file, "sync", "feature"); err == nil {
		t.Fatal("expected workspace error")
	}
}

func TestSyncProfileBranchAndEmptySet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	body := "root:\n  branch: from-root\nsync:\n  branch: feature\n"
	if err := os.WriteFile(filepath.Join(dir, "ok.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	git := readyGit()
	git.oid = "aaa"
	git.lsOID = "aaa"
	lines := install(t, git)
	ws := t.TempDir()
	if err := runCmd(t, "--profile", "ok", "--retries-number", "0", "--workspace", ws, "sync"); err != nil {
		t.Fatal(err)
	}
	if git.heads != 0 {
		t.Fatalf("empty workspace queried git %d", git.heads)
	}
	makeCopy(t, ws, "documentation/architecture")
	if err := runCmd(t, "--profile", "ok", "--retries-number", "0", "--workspace", ws, "sync", "other"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 0 || git.pulls != 0 {
		t.Fatalf("lines %v pulls %d", *lines, git.pulls)
	}
	git.detached = true
	if err := runCmd(t, "--profile", "ok", "--retries-number", "0", "--workspace", ws, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 0 || git.pulls != 0 {
		t.Fatalf("detached lines %v", *lines)
	}
	git.detached = false
	if err := runCmd(t, "--profile", "ok", "--retries-number", "0", "--workspace", ws, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "➡️ ➡️ documentation/architecture" {
		t.Fatalf("%v", *lines)
	}
}

func TestSyncRootCopyTokenAndLog(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := readyGit()
	git.remoteURL = "https://github.com/acme/api.git"
	git.oid = "local"
	git.oidAfterPull = "merge"
	git.lsOID = "remote"
	git.pullStderr = "see https://x-access-token:sekret@github.com/acme/api.git"
	lines := install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, ".")
	err := runCmd(t, "--retries-number", "0", "--git-https-token", "sekret", "--upstream", "github", "--workspace", ws, "sync", "feature", "--git-pull-opts=--ff-only")
	if err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ ✔️ ." {
		t.Fatalf("%v", *lines)
	}
	if git.lastRemote != "origin" || !strings.Contains(strings.Join(git.lastCfg, " "), "insteadOf=https://github.com/acme/api.git") {
		t.Fatalf("remote %s cfg %v", git.lastRemote, git.lastCfg)
	}
	path := cli.ActiveRuntime.Trace.FilePath()
	body, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	text := string(body)
	if strings.Contains(text, "sekret") || !strings.Contains(text, "***") {
		t.Fatalf("log %s", text)
	}
}

func TestSyncMassConfirm(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := readyGit()
	git.oid = "aaa"
	git.lsOID = "aaa"
	install(t, git)
	many := t.TempDir()
	for i := 0; i < 11; i++ {
		makeCopy(t, many, filepath.Join("n", string(rune('a'+i))))
	}
	prev := confirmMass
	confirmMass = func(*console.UI, int) (bool, error) { return false, nil }
	t.Cleanup(func() { confirmMass = prev })
	if err := runCmd(t, "--retries-number", "0", "--workspace", many, "sync", "feature"); err != nil {
		t.Fatal(err)
	}
	if git.pulls != 0 {
		t.Fatalf("declined run pulled %d", git.pulls)
	}
	called := false
	confirmMass = func(*console.UI, int) (bool, error) {
		called = true
		return false, nil
	}
	if err := runCmd(t, "--retries-number", "0", "--workspace", many, "sync", "feature", "--all"); err != nil {
		t.Fatal(err)
	}
	if called || git.pulls != 11 {
		t.Fatalf("called %v pulls %d", called, git.pulls)
	}
	confirmMass = func(*console.UI, int) (bool, error) {
		return false, errString("confirmation is not available without a terminal")
	}
	if err := runCmd(t, "--retries-number", "0", "--workspace", many, "sync", "feature"); err == nil {
		t.Fatal("expected confirm error")
	}
}

func TestSyncInteractiveCancelAndFailure(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := readyGit()
	install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	asPicker = func(*console.UI) workspace.Picker { return cancelPicker{} }
	if err := runCmd(t, "--workspace", ws, "sync", "feature", "--interactive"); err != nil {
		t.Fatal(err)
	}
	if git.pulls != 0 {
		t.Fatalf("cancelled pulled %d", git.pulls)
	}
	asPicker = func(*console.UI) workspace.Picker { return errPicker{} }
	if err := runCmd(t, "--workspace", ws, "sync", "feature", "--interactive"); err == nil {
		t.Fatal("expected interactive error")
	}
}

func TestSyncKeepError(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := readyGit()
	git.headErr = errString("git failed")
	install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	if err := runCmd(t, "--workspace", ws, "sync", "feature"); err == nil {
		t.Fatal("expected keep error")
	}
	if git.pulls != 0 {
		t.Fatal(git.pulls)
	}
}

type cancelPicker struct{}

func (cancelPicker) Select([]console.SelectItem) ([]int, error) {
	return nil, console.ErrSelectCancelled
}

type errPicker struct{}

func (errPicker) Select([]console.SelectItem) ([]int, error) {
	return nil, errString("interactive input is not available")
}
