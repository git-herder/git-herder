package publish

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Сквозной прогон publish: адаптеры SCM ходят в httptest, git подменён.
// t.Parallel не используется: runner — пакетная переменная.
func TestPublishIntegration(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())

	t.Run("gitlab", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, readHit(r))
			switch {
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/group%2Fapi":
				http.NotFound(w, r)
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/group":
				http.NotFound(w, r)
			case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/groups":
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":1}`))
			case r.Method == http.MethodPost && r.URL.EscapedPath() == "/api/v4/projects":
				w.WriteHeader(http.StatusCreated)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		ws := t.TempDir()
		makeCopy(t, ws, "api")
		git := stubRunner(t)
		err := runCmd(t,
			"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "publish", "group",
		)
		if err != nil {
			t.Fatal(err)
		}
		want := sshRemote(t, srv.URL, "group/api")
		if len(git.adds) != 1 || git.adds[0] != want {
			t.Fatalf("adds %v want %s", git.adds, want)
		}
		if len(git.pushes) != 1 || git.pushes[0] != "main" {
			t.Fatalf("pushes %v", git.pushes)
		}
		if !saw(hits, http.MethodGet, "/api/v4/projects/group%2Fapi") || !saw(hits, http.MethodPost, "/api/v4/projects") {
			t.Fatalf("hits %+v", hits)
		}
		assertBearer(t, hits)
	})

	t.Run("github", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, readHit(r))
			switch {
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/acme/api":
				http.NotFound(w, r)
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/orgs/acme":
				w.WriteHeader(http.StatusOK)
			case r.Method == http.MethodPost && r.URL.EscapedPath() == "/orgs/acme/repos":
				w.WriteHeader(http.StatusCreated)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		ws := t.TempDir()
		makeCopy(t, ws, "api")
		git := stubRunner(t)
		err := runCmd(t,
			"--upstream", "github", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "publish", "acme",
		)
		if err != nil {
			t.Fatal(err)
		}
		want := sshRemote(t, srv.URL, "acme/api")
		if len(git.adds) != 1 || git.adds[0] != want {
			t.Fatalf("adds %v want %s", git.adds, want)
		}
		if len(git.pushes) != 1 || git.pushes[0] != "main" {
			t.Fatalf("pushes %v", git.pushes)
		}
		if !sawBody(hits, http.MethodPost, "/orgs/acme/repos", `"name":"api"`) {
			t.Fatalf("hits %+v", hits)
		}
		assertBearer(t, hits)
	})

	t.Run("github path attribute", func(t *testing.T) {
		var hits []scmHit
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits = append(hits, readHit(r))
			switch {
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/orgs/acme":
				w.WriteHeader(http.StatusOK)
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/orgs/acme/properties/schema/workspace-path":
				_, _ = w.Write([]byte(`{"value_type":"string"}`))
			case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/acme/domain-billing-api":
				http.NotFound(w, r)
			case r.Method == http.MethodPost && r.URL.EscapedPath() == "/orgs/acme/repos":
				w.WriteHeader(http.StatusCreated)
			case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/repos/acme/domain-billing-api/properties/values":
				w.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		ws := t.TempDir()
		makeCopy(t, ws, "domain/billing/api")
		git := stubRunner(t)
		err := runCmd(t,
			"--upstream", "github", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
			"--workspace", ws, "publish", "acme", "--path-attribute", "workspace-path",
		)
		if err != nil {
			t.Fatal(err)
		}
		want := sshRemote(t, srv.URL, "acme/domain-billing-api")
		if len(git.adds) != 1 || git.adds[0] != want {
			t.Fatalf("adds %v want %s", git.adds, want)
		}
		if len(git.pushes) != 1 || git.pushes[0] != "main" {
			t.Fatalf("pushes %v", git.pushes)
		}
		if !saw(hits, http.MethodGet, "/orgs/acme/properties/schema/workspace-path") {
			t.Fatalf("hits %+v", hits)
		}
		if !sawBody(hits, http.MethodPatch, "/repos/acme/domain-billing-api/properties/values", `"value":"domain/billing/api"`) {
			t.Fatalf("hits %+v", hits)
		}
		assertBearer(t, hits)
	})
}

type scmHit struct {
	method string
	path   string
	auth   string
	body   string
}

func readHit(r *http.Request) scmHit {
	b, _ := io.ReadAll(r.Body)
	return scmHit{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization"), body: string(b)}
}

func stubRunner(t *testing.T) *fakeGit {
	t.Helper()
	prev := runner
	git := &fakeGit{head: "main", branches: map[string]bool{"main": true}}
	runner = git
	t.Cleanup(func() { runner = prev })
	return git
}

func sshRemote(t *testing.T, raw, path string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return "ssh://git@" + u.Hostname() + ":" + u.Port() + "/" + path + ".git"
}

func saw(hits []scmHit, method, path string) bool {
	for _, h := range hits {
		if h.method == method && h.path == path {
			return true
		}
	}
	return false
}

func sawBody(hits []scmHit, method, path, fragment string) bool {
	for _, h := range hits {
		if h.method == method && h.path == path && strings.Contains(h.body, fragment) {
			return true
		}
	}
	return false
}

func assertBearer(t *testing.T, hits []scmHit) {
	t.Helper()
	if len(hits) == 0 {
		t.Fatal("no scm requests")
	}
	for _, h := range hits {
		if h.auth != "Bearer tok" {
			t.Fatalf("auth %q on %s %s", h.auth, h.method, h.path)
		}
	}
}
