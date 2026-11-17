package clone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
)

func TestCloneCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case strings.Contains(r.URL.Path, "/labels"):
			_, _ = w.Write([]byte(`[{"name":"pub"}]`))
		case strings.Contains(r.URL.Path, "/custom_attributes"):
			_, _ = w.Write([]byte(`[]`))
		default:
			w.Header().Set("X-Total-Pages", "1")
			_, _ = w.Write([]byte(`[
				{"id":1,"name":"api","path_with_namespace":"g/api","web_url":"https://gitlab.example/g/api"}
			]`))
		}
	}))
	defer srv.Close()

	ws := filepath.Join(dir, "ws")
	prev := runGit
	runGit = func(context.Context, []string, string, string) (string, error) {
		return "", nil
	}
	t.Cleanup(func() { runGit = prev })

	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0", "--workspace", ws, "clone"})
	if err := root.Execute(); err == nil {
		t.Fatal("missing criterion")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "clone", "--qn", "api", "--git-opts=--separate-git-dir=/tmp/x"})
	if err := root.Execute(); err == nil {
		t.Fatal("forbidden git opt")
	}
	if hits != 0 {
		t.Fatalf("scm called %d", hits)
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--workspace", ws, "clone", "--qn", "missing", "--interactive"})
	if err := root.Execute(); err == nil {
		t.Fatal("interactive without tty")
	}

	var cloned []string
	runGit = func(_ context.Context, opts []string, repoURL, dest string) (string, error) {
		cloned = append(cloned, repoURL+" -> "+dest)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", err
		}
		return "", nil
	}
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
		"--workspace", ws, "clone", "--qn", "api", "--git-opts=--depth=1",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(cloned) != 1 || !strings.Contains(cloned[0], "git@gitlab.example:g/api.git") {
		t.Fatalf("cloned=%v", cloned)
	}
	if _, err := os.Stat(filepath.Join(ws, "g", "api")); err != nil {
		t.Fatal(err)
	}

	emptyWS := filepath.Join(dir, "empty-ws")
	hits = 0
	srvEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("X-Total-Pages", "1")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srvEmpty.Close()
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srvEmpty.URL, "--token", "tok", "--workspace", emptyWS, "clone", "--qn", "api"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(emptyWS); !os.IsNotExist(err) {
		t.Fatal("workspace created for empty selection")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--token", "tok", "clone", "--qn", "api"})
	if err := root.Execute(); err == nil {
		t.Fatal("missing upstream")
	}
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "clone", "--qn", "api"})
	if err := root.Execute(); err == nil {
		t.Fatal("missing token")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "github", "--host", "https://api.github.com", "--token", "tok",
		"--workspace", filepath.Join(dir, "attr-ws"),
		"clone", "--qn", "api", "--path-attribute", "workspace-path", "--preserve-path",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("path-attribute with preserve-path")
	}
}
