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

// Сквозной прогон clone: адаптеры SCM ходят в httptest, git clone подменён.
// t.Parallel не используется: runGit — пакетная переменная.
func TestCloneIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)

	t.Run("gitlab", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, scmHit{path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")})
			if r.URL.Path != "/api/v4/projects" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("X-Total-Pages", "1")
			_, _ = w.Write([]byte(`[{"id":1,"name":"api","path_with_namespace":"g/api","web_url":"https://gitlab.example/g/api"}]`))
		}))
		t.Cleanup(srv.Close)

		ws := filepath.Join(dir, "gitlab-ws")
		calls := stubRunGit(t)
		err := runClone(t,
			"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "clone", "--qn", "api",
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 1 {
			t.Fatalf("clones=%d", len(*calls))
		}
		wantDest := filepath.Join(ws, "g", "api")
		if (*calls)[0].url != "git@gitlab.example:g/api.git" || (*calls)[0].dest != wantDest {
			t.Fatalf("clone %+v", (*calls)[0])
		}
		if _, err := os.Stat(wantDest); err != nil {
			t.Fatal(err)
		}
		if len(hits) == 0 || hits[0].path != "/api/v4/projects" || hits[0].auth != "Bearer tok" {
			t.Fatalf("hits %+v", hits)
		}
	})

	t.Run("github", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, scmHit{path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")})
			if r.URL.Path != "/search/repositories" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"total_count":1,"items":[{"full_name":"acme/api","name":"api","html_url":"https://github.example/acme/api","topics":[]}]}`))
		}))
		t.Cleanup(srv.Close)

		ws := filepath.Join(dir, "github-ws")
		calls := stubRunGit(t)
		err := runClone(t,
			"--upstream", "github", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "clone", "--qn", "api",
		)
		if err != nil {
			t.Fatal(err)
		}
		wantDest := filepath.Join(ws, "acme", "api")
		if len(*calls) != 1 || (*calls)[0].url != "git@github.example:acme/api.git" || (*calls)[0].dest != wantDest {
			t.Fatalf("clone %+v", *calls)
		}
		if _, err := os.Stat(wantDest); err != nil {
			t.Fatal(err)
		}
		if !sawQuery(hits, "/search/repositories", "q=api") {
			t.Fatalf("hits %+v", hits)
		}
		if hits[0].auth != "Bearer tok" {
			t.Fatalf("auth %q", hits[0].auth)
		}
	})

	t.Run("github path attribute", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, scmHit{path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")})
			switch r.URL.Path {
			case "/search/repositories":
				_, _ = w.Write([]byte(`{"total_count":1,"items":[{"full_name":"acme/api","name":"api","html_url":"https://github.example/acme/api","topics":[]}]}`))
			case "/repos/acme/api/properties/values":
				_, _ = w.Write([]byte(`[{"property_name":"workspace-path","value":"domain/billing/api"}]`))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		ws := filepath.Join(dir, "attr-ws")
		calls := stubRunGit(t)
		err := runClone(t,
			"--upstream", "github", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "clone", "--qn", "api", "--path-attribute", "workspace-path",
		)
		if err != nil {
			t.Fatal(err)
		}
		wantDest := filepath.Join(ws, "domain", "billing", "api")
		if len(*calls) != 1 || (*calls)[0].url != "git@github.example:acme/api.git" || (*calls)[0].dest != wantDest {
			t.Fatalf("clone %+v", *calls)
		}
		if _, err := os.Stat(wantDest); err != nil {
			t.Fatal(err)
		}
		if !sawPath(hits, "/repos/acme/api/properties/values") {
			t.Fatalf("hits %+v", hits)
		}
	})
}

type scmHit struct {
	path  string
	query string
	auth  string
}

type cloneCall struct {
	url  string
	dest string
}

func stubRunGit(t *testing.T) *[]cloneCall {
	t.Helper()
	prev := runGit
	var calls []cloneCall
	runGit = func(_ context.Context, _ []string, repoURL, dest string) (string, error) {
		calls = append(calls, cloneCall{url: repoURL, dest: dest})
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", err
		}
		return "", nil
	}
	t.Cleanup(func() { runGit = prev })
	return &calls
}

func runClone(t *testing.T, args ...string) error {
	t.Helper()
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs(args)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root.Execute()
}

func sawQuery(hits []scmHit, path, fragment string) bool {
	for _, h := range hits {
		if h.path == path && strings.Contains(h.query, fragment) {
			return true
		}
	}
	return false
}

func sawPath(hits []scmHit, path string) bool {
	for _, h := range hits {
		if h.path == path {
			return true
		}
	}
	return false
}
