package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/conn"
	gh "github.com/git-herder/git-herder/internal/scm/github"
)

func install(t *testing.T, git gitOps, repo scm.RepositoryCreator) *[]string {
	t.Helper()
	prevR, prevC, prevP := runner, openCreator, printLine
	var lines []string
	runner = git
	printLine = func(s string) { lines = append(lines, s) }
	if repo != nil {
		openCreator = func(conn.Config) (scm.RepositoryCreator, error) { return repo, nil }
	}
	t.Cleanup(func() {
		runner = prevR
		openCreator = prevC
		printLine = prevP
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

func TestPublishRejectsBeforeSearch(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	repo := &fakeRepo{}
	install(t, &fakeGit{head: "main", branches: map[string]bool{"main": true}}, repo)
	ws := t.TempDir()
	if err := runCmd(t, "--upstream", "github", "--token", "tok", "--workspace", ws, "publish"); err == nil || !strings.Contains(err.Error(), "root is required") {
		t.Fatal(err)
	}
	if err := runCmd(t, "--upstream", "github", "--token", "tok", "--workspace", ws, "publish", "acme", "--interactive", "--all"); err == nil {
		t.Fatal("expected interactive and all error")
	}
	if err := runCmd(t, "--upstream", "gitlab", "--token", "tok", "--workspace", ws, "publish", "group", "--leaf-name"); err == nil {
		t.Fatal("expected leaf-name error")
	}
	if err := runCmd(t, "--upstream", "github", "--token", "tok", "--workspace", ws, "publish", "acme/team"); err == nil {
		t.Fatal("expected flat root error")
	}
	if err := runCmd(t, "--token", "tok", "--workspace", ws, "publish", "acme"); err == nil {
		t.Fatal("expected upstream error")
	}
	if len(repo.calls) != 0 || repo.preCalls != 0 {
		t.Fatalf("scm called %v pre %d", repo.calls, repo.preCalls)
	}
}

func TestPublishEmptyAndExcluded(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	repo := &fakeRepo{}
	ws := t.TempDir()
	drop := filepath.Join(ws, "old")
	makeCopy(t, ws, "old")
	makeCopy(t, ws, "fresh")
	git := &fakeGit{
		head:     "main",
		branches: map[string]bool{"main": true},
		urlsOf: func(dir string) []string {
			if dir == drop {
				return []string{"git@github.com:acme/old.git"}
			}
			return nil
		},
	}
	lines := install(t, git, repo)
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "0", "--workspace", ws, "publish", "acme"); err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) != 2 || !strings.Contains(repo.calls[1], "create acme/fresh") {
		t.Fatalf("calls %v", repo.calls)
	}
	if len(git.adds) != 1 || git.adds[0] != "git@github.com:acme/fresh.git" {
		t.Fatalf("adds %v", git.adds)
	}
	if len(*lines) != 1 || !strings.HasPrefix((*lines)[0], "✔ ") {
		t.Fatalf("lines %v", *lines)
	}

	repo.calls = nil
	empty := t.TempDir()
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--workspace", empty, "publish", "acme"); err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("empty selection called scm %v", repo.calls)
	}
}

func TestPublishDuplicateDetachedAndThreshold(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	repo := &fakeRepo{}
	git := &fakeGit{head: "main", branches: map[string]bool{"main": true}}
	lines := install(t, git, repo)
	ws := t.TempDir()
	makeCopy(t, ws, "a/b-c")
	makeCopy(t, ws, "a-b/c")
	err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--workspace", ws, "publish", "acme")
	if err == nil || !strings.Contains(err.Error(), "duplicate repository name") {
		t.Fatal(err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("duplicate created %v", repo.calls)
	}

	ws = t.TempDir()
	makeCopy(t, ws, "solo")
	git.detached = true
	*lines = nil
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--workspace", ws, "publish", "acme"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || !strings.HasPrefix((*lines)[0], "➡️ ") || len(repo.calls) != 0 {
		t.Fatalf("lines %v calls %v", *lines, repo.calls)
	}

	git.detached = false
	many := t.TempDir()
	for i := 0; i < 11; i++ {
		makeCopy(t, many, filepath.Join("n", string(rune('a'+i))))
	}
	prev := confirmMass
	confirmMass = func(*console.UI, int) (bool, error) { return false, nil }
	t.Cleanup(func() { confirmMass = prev })
	repo.calls = nil
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "0", "--workspace", many, "publish", "acme"); err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("refused run created %v", repo.calls)
	}
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "0", "--workspace", many, "publish", "acme", "--all"); err != nil {
		t.Fatal(err)
	}
	creates := 0
	for _, c := range repo.calls {
		if strings.HasPrefix(c, "create ") {
			creates++
		}
	}
	if creates != 11 {
		t.Fatalf("creates %d calls %v", creates, repo.calls)
	}
}

func TestPublishProfileRootPreflightAndGitLab(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	repo := &fakeRepo{}
	git := &fakeGit{head: "main", branches: map[string]bool{"main": true}}
	install(t, git, repo)
	ws := t.TempDir()
	ignored := "root:\n  upstream: github\n  host: https://api.github.com\n  token: tok\n  root: acme\n"
	if err := os.WriteFile(filepath.Join(dir, "ign.yaml"), []byte(ignored), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, "--profile", "ign", "--workspace", ws, "publish"); err == nil {
		t.Fatal("global root key must not supply publish root")
	}

	prof := "root:\n  upstream: github\n  host: https://api.github.com\n  token: tok\npublish:\n  root: acme\n"
	if err := os.WriteFile(filepath.Join(dir, "ok.yaml"), []byte(prof), 0o600); err != nil {
		t.Fatal(err)
	}
	makeCopy(t, ws, "team/api")
	if err := runCmd(t, "--profile", "ok", "--retries-number", "0", "--workspace", ws, "publish"); err != nil {
		t.Fatal(err)
	}
	if len(git.adds) != 1 || git.adds[0] != "git@github.com:acme/team-api.git" {
		t.Fatalf("adds %v", git.adds)
	}

	repo.preflight = []error{gh.ErrOrgNotFound}
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "5", "--workspace", ws, "publish", "acme", "--path-attribute", "workspace-path"); err == nil {
		t.Fatal("expected preflight error")
	}
	if repo.preCalls != 1 {
		t.Fatalf("preflight calls %d", repo.preCalls)
	}

	repo.preflight = []error{&scmHTTP{status: 500}}
	repo.preCalls = 0
	repo.writes = nil
	empty := t.TempDir()
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "1", "--workspace", empty, "publish", "acme", "--path-attribute", "workspace-path"); err != nil {
		t.Fatal(err)
	}
	if repo.preCalls != 2 {
		t.Fatalf("preflight retries %d", repo.preCalls)
	}

	gl := t.TempDir()
	makeCopy(t, gl, "domain/billing/api")
	git.adds = nil
	git.remote = nil
	repo.calls = nil
	if err := runCmd(t, "--upstream", "gitlab", "--host", "https://gitlab.example", "--token", "tok", "--retries-number", "0", "--workspace", gl, "publish", "group/sub"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(repo.calls, "\n")
	if !strings.Contains(joined, "create group/sub/domain/billing/api") {
		t.Fatalf("calls %v", repo.calls)
	}
	if len(git.adds) != 1 || git.adds[0] != "git@gitlab.example:group/sub/domain/billing/api.git" {
		t.Fatalf("adds %v", git.adds)
	}

	attrWS := t.TempDir()
	makeCopy(t, attrWS, "domain/billing/api")
	repo.writes = nil
	git.adds = nil
	git.remote = nil
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--git-https-token", "secret", "--retries-number", "0", "--workspace", attrWS, "publish", "acme", "--protocol", "https", "--path-attribute", "workspace-path"); err != nil {
		t.Fatal(err)
	}
	if len(repo.writes) != 1 || repo.writes[0] != "domain/billing/api" {
		t.Fatalf("writes %v", repo.writes)
	}
	if len(git.adds) != 1 || git.adds[0] != "https://x-access-token:secret@github.com/acme/domain-billing-api" {
		t.Fatalf("adds %v", git.adds)
	}
}

type scmHTTP struct{ status int }

func (e *scmHTTP) Error() string { return "http" }

func TestPublishSkipAndNoTerminalThreshold(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	repo := &fakeRepo{}
	git := &fakeGit{
		head:     "main",
		branches: map[string]bool{"main": true},
		remote:   map[string]string{"origin": "https://elsewhere.example/acme/solo.git"},
	}
	lines := install(t, git, repo)
	ws := t.TempDir()
	makeCopy(t, ws, "solo")
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "0", "--workspace", ws, "publish", "acme", "--existing", "skip"); err != nil {
		t.Fatal(err)
	}
	if len(git.pushes) != 0 || len(*lines) != 1 || !strings.HasPrefix((*lines)[0], "➡️ ") {
		t.Fatalf("pushes %v lines %v", git.pushes, *lines)
	}

	many := t.TempDir()
	for i := 0; i < 11; i++ {
		makeCopy(t, many, string(rune('a'+i)))
	}
	git.remote = nil
	repo.calls = nil
	if err := runCmd(t, "--upstream", "github", "--host", "https://api.github.com", "--token", "tok", "--retries-number", "0", "--workspace", many, "publish", "acme"); err == nil {
		t.Fatal("expected missing terminal")
	}
	if len(repo.calls) != 0 {
		t.Fatalf("threshold created %v", repo.calls)
	}
}
