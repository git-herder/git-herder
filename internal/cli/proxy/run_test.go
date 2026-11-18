package proxy_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/bootstrap"
	"github.com/git-herder/git-herder/internal/cli/proxy"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/version"
)

type fakeGit struct {
	calls []call
	fn    func(args []string) (gitexec.Result, error)
}

type call struct {
	dir  string
	args []string
}

func (f *fakeGit) Run(_ context.Context, dir string, args []string) (gitexec.Result, error) {
	f.calls = append(f.calls, call{dir: dir, args: append([]string{}, args...)})
	if f.fn == nil {
		return gitexec.Result{}, nil
	}
	return f.fn(args)
}

func (f *fakeGit) userCalls(verb string) []call {
	var out []call
	for _, c := range f.calls {
		if hasVerb(c.args, verb) {
			out = append(out, c)
		}
	}
	return out
}

func hasVerb(args []string, verb string) bool {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if a == "-c" {
			skip = true
			continue
		}
		if a == verb {
			return true
		}
	}
	return false
}

func setup(t *testing.T) (string, *fakeGit, *[]string) {
	t.Helper()
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	ws := t.TempDir()
	git := &fakeGit{}
	t.Cleanup(proxy.InstallRunnerForTest(git))
	var lines []string
	t.Cleanup(proxy.InstallIOForTest(func(s string) { lines = append(lines, s) }, nil))
	return ws, git, &lines
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

func TestPullPassesTailAndRelativePath(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "documentation/architecture")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull", "--rebase", "origin", "main"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := git.userCalls("pull")
	if len(got) != 1 {
		t.Fatalf("calls=%v", git.calls)
	}
	want := []string{"pull", "--rebase", "origin", "main"}
	if strings.Join(got[0].args, " ") != strings.Join(want, " ") {
		t.Fatalf("args=%v", got[0].args)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ documentation/architecture" {
		t.Fatalf("lines=%q", *lines)
	}
}

func TestVerbsAndCluster(t *testing.T) {
	ws, git, _ := setup(t)
	makeCopy(t, ws, "api")
	for _, verb := range []string{"push", "fetch", "remote"} {
		git.calls = nil
		code := bootstrap.RunWithArgs([]string{"--workspace", ws, verb, "-iA"})
		if code != 0 {
			t.Fatalf("%s code=%d", verb, code)
		}
		got := git.userCalls(verb)
		if len(got) != 1 || strings.Join(got[0].args, " ") != verb+" -iA" {
			t.Fatalf("%s args=%v", verb, git.calls)
		}
	}
}

func TestFailureUsesStderrThenStdout(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	git.fn = func(args []string) (gitexec.Result, error) {
		return gitexec.Result{Exit: 1, Stderr: "nope\n", Stdout: "hidden"}, fmt.Errorf("git pull")
	}
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if len(*lines) != 1 || (*lines)[0] != "❌ api\n    nope" {
		t.Fatalf("lines=%q", *lines)
	}
	*lines = nil
	git.fn = func(args []string) (gitexec.Result, error) {
		return gitexec.Result{Exit: 1, Stdout: "only-out\n"}, fmt.Errorf("git pull")
	}
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if (*lines)[0] != "❌ api\n    only-out" {
		t.Fatalf("lines=%q", *lines)
	}
	*lines = nil
	git.fn = func(args []string) (gitexec.Result, error) {
		return gitexec.Result{}, fmt.Errorf("git not found")
	}
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if (*lines)[0] != "❌ api\n    git failed" {
		t.Fatalf("lines=%q", *lines)
	}
}

func TestOnErrorAbortAndContinue(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "a")
	makeCopy(t, ws, "b")
	git.fn = func(args []string) (gitexec.Result, error) {
		return gitexec.Result{Exit: 1, Stderr: "x"}, fmt.Errorf("git")
	}
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "--on-error", "abort", "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if len(git.userCalls("pull")) != 1 || len(*lines) != 1 {
		t.Fatalf("calls=%d lines=%d", len(git.userCalls("pull")), len(*lines))
	}
	git.calls = nil
	*lines = nil
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "--on-error", "continue", "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if len(git.userCalls("pull")) != 2 || len(*lines) != 2 {
		t.Fatalf("calls=%d lines=%d", len(git.userCalls("pull")), len(*lines))
	}
}

func TestAllNoticeAndTailAll(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "fetch", "--all"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if len(*lines) != 2 || (*lines)[0] != "Not passed to git: --all" {
		t.Fatalf("lines=%q", *lines)
	}
	if strings.Join(git.userCalls("fetch")[0].args, " ") != "fetch" {
		t.Fatalf("args=%v", git.calls)
	}
	git.calls = nil
	*lines = nil
	code = bootstrap.RunWithArgs([]string{"--workspace", ws, "fetch", "-A", "--", "--all"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ api" {
		t.Fatalf("lines=%q", *lines)
	}
	if strings.Join(git.userCalls("fetch")[0].args, " ") != "fetch --all" {
		t.Fatalf("args=%v", git.calls)
	}
}

func TestHTTPSTokenPairsAndDedup(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	git.fn = func(args []string) (gitexec.Result, error) {
		switch strings.Join(args, " ") {
		case "remote":
			return gitexec.Result{Stdout: "origin\nupstream\n"}, nil
		case "remote get-url origin", "remote get-url upstream":
			return gitexec.Result{Stdout: "https://github.com/acme/api.git\n"}, nil
		case "remote get-url --push origin":
			return gitexec.Result{Stdout: "https://github.com/acme/api.git\n"}, nil
		case "remote get-url --push upstream":
			return gitexec.Result{Stdout: "git@github.com:acme/api.git\n"}, nil
		default:
			return gitexec.Result{}, nil
		}
	}
	code := bootstrap.RunWithArgs([]string{
		"--workspace", ws, "--git-https-token", "sekret", "--upstream", "github", "pull", "--ff-only",
	})
	if code != 0 {
		t.Fatalf("code=%d lines=%q", code, *lines)
	}
	users := git.userCalls("pull")
	if len(users) != 1 {
		t.Fatalf("calls=%v", git.calls)
	}
	joined := strings.Join(users[0].args, " ")
	if !strings.Contains(joined, "insteadOf=https://github.com/acme/api.git") || !strings.Contains(joined, "x-access-token:sekret@") {
		t.Fatalf("args=%s", joined)
	}
	if !strings.Contains(joined, "insteadOf=git@github.com:acme/api.git") {
		t.Fatalf("args=%s", joined)
	}
	if strings.Count(joined, "insteadOf=") != 2 {
		t.Fatalf("dedup args=%s", joined)
	}
	if !strings.HasSuffix(joined, "pull --ff-only") {
		t.Fatalf("tail args=%s", joined)
	}
	httpsAt := strings.Index(joined, "insteadOf=https://github.com/acme/api.git")
	sshAt := strings.Index(joined, "insteadOf=git@github.com:acme/api.git")
	if httpsAt < 0 || sshAt < httpsAt {
		t.Fatalf("order args=%s", joined)
	}
	for _, c := range git.calls {
		if hasVerb(c.args, "pull") {
			continue
		}
		if strings.Contains(strings.Join(c.args, " "), "-c") {
			t.Fatalf("prep got -c: %v", c.args)
		}
	}
}

func TestRemoteReadFailureAndBadURL(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	git.fn = func(args []string) (gitexec.Result, error) {
		if strings.Join(args, " ") == "remote" {
			return gitexec.Result{Stdout: "origin\n"}, nil
		}
		return gitexec.Result{Exit: 2, Stderr: "no url\n"}, fmt.Errorf("git")
	}
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "--git-https-token", "sekret", "push"})
	if code != 1 || len(git.userCalls("push")) != 0 {
		t.Fatalf("code=%d calls=%v", code, git.calls)
	}
	if (*lines)[0] != "❌ api\n    no url" {
		t.Fatalf("lines=%q", *lines)
	}
	git.calls = nil
	*lines = nil
	git.fn = func(args []string) (gitexec.Result, error) {
		if strings.Join(args, " ") == "remote" {
			return gitexec.Result{Stdout: "origin\n"}, nil
		}
		if strings.HasPrefix(strings.Join(args, " "), "remote get-url") {
			return gitexec.Result{Stdout: "file:///tmp/repo.git\n"}, nil
		}
		return gitexec.Result{}, nil
	}
	code = bootstrap.RunWithArgs([]string{"--workspace", ws, "--git-https-token", "sekret", "push"})
	if code != 1 || len(git.userCalls("push")) != 0 {
		t.Fatalf("code=%d calls=%v", code, git.calls)
	}
	if (*lines)[0] != "❌ api\n    git failed" {
		t.Fatalf("lines=%q", *lines)
	}
}

func TestBlankTokenSkipsRemoteRead(t *testing.T) {
	ws, git, _ := setup(t)
	makeCopy(t, ws, "api")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "--git-https-token", "   ", "pull"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if len(git.calls) != 1 || strings.Join(git.calls[0].args, " ") != "pull" {
		t.Fatalf("calls=%v", git.calls)
	}
}

func TestEmptySelectionAndBadWorkspace(t *testing.T) {
	ws, git, lines := setup(t)
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"}); code != 0 || len(git.calls) != 0 || len(*lines) != 0 {
		t.Fatalf("code=%d calls=%d lines=%q", code, len(git.calls), *lines)
	}
	file := filepath.Join(ws, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := bootstrap.RunWithArgs([]string{"--workspace", file, "pull"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestAllAndInteractive(t *testing.T) {
	ws, git, _ := setup(t)
	if code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull", "--all", "--interactive"}); code != 1 || len(git.calls) != 0 {
		t.Fatalf("code=%d calls=%d", code, len(git.calls))
	}
}

func TestProfileFiltersWithoutNotice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "default"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := "pull:\n  qn: api\n  all: true\n"
	if err := os.WriteFile(filepath.Join(dir, "work.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	git := &fakeGit{}
	t.Cleanup(proxy.InstallRunnerForTest(git))
	var lines []string
	t.Cleanup(proxy.InstallIOForTest(func(s string) { lines = append(lines, s) }, nil))
	makeCopy(t, ws, "api")
	makeCopy(t, ws, "billing")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if len(git.userCalls("pull")) != 1 || len(lines) != 1 || lines[0] != "✔️ api" {
		t.Fatalf("calls=%v lines=%q", git.calls, lines)
	}
	git.calls = nil
	lines = nil
	code = bootstrap.RunWithArgs([]string{"--workspace", ws, "pull", "--qn", "billing"})
	if code != 0 || len(git.userCalls("pull")) != 1 || len(lines) != 1 || lines[0] != "✔️ billing" {
		t.Fatalf("code=%d calls=%v lines=%q", code, git.calls, lines)
	}
}

func TestAllTokensAndEmptyRemotes(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "fetch", "--all", "-A"})
	if code != 0 || len(*lines) < 1 || (*lines)[0] != "Not passed to git: --all -A" {
		t.Fatalf("code=%d lines=%q", code, *lines)
	}
	if strings.Join(git.userCalls("fetch")[0].args, " ") != "fetch" {
		t.Fatalf("args=%v", git.calls)
	}
	git.calls = nil
	*lines = nil
	git.fn = func(args []string) (gitexec.Result, error) {
		if strings.Join(args, " ") == "remote" {
			return gitexec.Result{Stdout: "\n"}, nil
		}
		return gitexec.Result{Stderr: "noise\n"}, nil
	}
	code = bootstrap.RunWithArgs([]string{"--workspace", ws, "--git-https-token", "sekret", "fetch"})
	if code != 0 || len(*lines) != 1 || (*lines)[0] != "✔️ api" {
		t.Fatalf("code=%d lines=%q calls=%v", code, *lines, git.calls)
	}
	if strings.Join(git.userCalls("fetch")[0].args, " ") != "fetch" {
		t.Fatalf("args=%v", git.calls)
	}
	sawRemote := false
	for _, c := range git.calls {
		if strings.Join(c.args, " ") == "remote" {
			sawRemote = true
		}
	}
	if !sawRemote {
		t.Fatalf("remote was not read: %v", git.calls)
	}
}

func TestThresholdRefusal(t *testing.T) {
	ws, git, lines := setup(t)
	for i := 0; i < 11; i++ {
		makeCopy(t, ws, fmt.Sprintf("r%02d", i))
	}
	asked := 0
	t.Cleanup(proxy.InstallIOForTest(nil, func(_ *console.UI, verb string, n int) (bool, error) {
		asked++
		if verb != "pull" || n != 11 {
			t.Errorf("prompt %s %d", verb, n)
		}
		return false, nil
	}))
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "pull"})
	if code != 0 || asked != 1 || len(git.calls) != 0 || len(*lines) != 0 {
		t.Fatalf("code=%d asked=%d calls=%d lines=%q", code, asked, len(git.calls), *lines)
	}
}

func TestVersionHelpAndMissingValue(t *testing.T) {
	ws, git, _ := setup(t)
	makeCopy(t, ws, "api")
	out := captureStdout(t, func() {
		if code := bootstrap.RunWithArgs([]string{"remote", "-v"}); code != 0 {
			t.Fatalf("code=%d", code)
		}
	})
	if out != "githerd version "+version.Semver+"\n" {
		t.Fatalf("version=%q", out)
	}
	if len(git.calls) != 0 {
		t.Fatalf("version ran git: %v", git.calls)
	}
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "remote", "--", "-v"})
	if code != 0 {
		t.Fatalf("git -v code=%d", code)
	}
	got := git.userCalls("remote")
	if len(got) != 1 || strings.Join(got[0].args, " ") != "remote -v" {
		t.Fatalf("args=%v", git.calls)
	}
	if code := bootstrap.RunWithArgs([]string{"pull", "-h"}); code != 0 {
		t.Fatalf("help code=%d", code)
	}
	if code := bootstrap.RunWithArgs([]string{"pull", "-w"}); code != 2 {
		t.Fatalf("missing code=%d", code)
	}
}

func TestTailExpandsPerCopy(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "billing/api")
	makeCopy(t, ws, "billing/web")
	code := bootstrap.RunWithArgs([]string{
		"--workspace", ws, "remote", "set-url", "origin",
		"git@github.com:myorg/{name}.git", "group/{path}/{foo}",
	})
	if code != 0 {
		t.Fatalf("code=%d lines=%q", code, *lines)
	}
	got := map[string]string{}
	for _, c := range git.userCalls("remote") {
		got[filepath.Base(c.dir)] = strings.Join(c.args, " ")
	}
	if got["api"] != "remote set-url origin git@github.com:myorg/api.git group/billing/api/{foo}" {
		t.Fatalf("api=%q", got["api"])
	}
	if got["web"] != "remote set-url origin git@github.com:myorg/web.git group/billing/web/{foo}" {
		t.Fatalf("web=%q", got["web"])
	}
	if len(*lines) != 2 {
		t.Fatalf("lines=%q", *lines)
	}
}

func TestEmptyRelativePathSkipsGit(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "remote", "add", "origin", "{path}.git"})
	if code != 1 || len(git.calls) != 0 {
		t.Fatalf("code=%d calls=%v", code, git.calls)
	}
	if len(*lines) != 1 || (*lines)[0] != "❌ .\n    empty relative path" {
		t.Fatalf("lines=%q", *lines)
	}
}

func TestRootNameExpandsWithoutPath(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "")
	code := bootstrap.RunWithArgs([]string{"--workspace", ws, "remote", "add", "origin", "{name}"})
	if code != 0 {
		t.Fatalf("code=%d lines=%q", code, *lines)
	}
	got := git.userCalls("remote")
	want := "remote add origin " + filepath.Base(ws)
	if len(got) != 1 || strings.Join(got[0].args, " ") != want {
		t.Fatalf("args=%v want %s", git.calls, want)
	}
}

func TestTokenPairsStayLiteralWhileTailExpands(t *testing.T) {
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	git.fn = func(args []string) (gitexec.Result, error) {
		joined := strings.Join(args, " ")
		if joined == "remote" || strings.HasPrefix(joined, "remote get-url") {
			return gitexec.Result{Stdout: "https://example.com/{name}.git\n"}, nil
		}
		return gitexec.Result{}, nil
	}
	code := bootstrap.RunWithArgs([]string{
		"--workspace", ws, "--git-https-token", "sekret", "--upstream", "github",
		"remote", "set-url", "origin", "{name}",
	})
	if code != 0 {
		t.Fatalf("code=%d lines=%q calls=%v", code, *lines, git.calls)
	}
	var joined string
	for _, c := range git.calls {
		line := strings.Join(c.args, " ")
		if strings.Contains(line, "set-url") {
			joined = line
		}
	}
	if joined == "" {
		t.Fatalf("calls=%v", git.calls)
	}
	if !strings.Contains(joined, "insteadOf=https://example.com/{name}.git") {
		t.Fatalf("pair expanded: %s", joined)
	}
	if !strings.HasSuffix(joined, "remote set-url origin api") {
		t.Fatalf("tail=%s", joined)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = prev
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
