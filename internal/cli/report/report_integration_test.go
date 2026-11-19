package report

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
)

// Сквозной прогон report: адаптеры SCM ходят в httptest и пишут JSON.
func TestReportIntegration(t *testing.T) {
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
		body := runReport(t, dir, "gitlab-out.json",
			"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"report", "--qn", "api",
		)
		if !strings.Contains(body, `"name":"api"`) {
			t.Fatalf("%s", body)
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
		body := runReport(t, dir, "github-out.json",
			"--upstream", "github", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"report", "--qn", "api",
		)
		if !strings.Contains(body, `"name":"api"`) {
			t.Fatalf("%s", body)
		}
		if !sawQuery(hits, "/search/repositories", "q=api") || hits[0].auth != "Bearer tok" {
			t.Fatalf("hits %+v", hits)
		}
	})
}

type scmHit struct {
	path  string
	query string
	auth  string
}

func runReport(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs(append(args, "--output", out))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func sawQuery(hits []scmHit, path, fragment string) bool {
	for _, h := range hits {
		if h.path == path && strings.Contains(h.query, fragment) {
			return true
		}
	}
	return false
}
