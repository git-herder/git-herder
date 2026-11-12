package report

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/cli"
)

func TestReportCommandPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/labels"):
			_, _ = w.Write([]byte(`[{"name":"pub"}]`))
		case strings.Contains(r.URL.Path, "/custom_attributes"):
			_, _ = w.Write([]byte(`[{"key":"tier","value":"gold"}]`))
		default:
			w.Header().Set("X-Total-Pages", "1")
			_, _ = w.Write([]byte(`[
				{"id":1,"name":"api","path_with_namespace":"g/api","web_url":"https://gitlab.example/g/api"},
				{"id":2,"name":"web","path_with_namespace":"g/web","web_url":"https://gitlab.example/g/web"}
			]`))
		}
	}))
	defer srv.Close()

	out := filepath.Join(dir, "out.json")
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
		"report", "--qn", "api", "--qt", "secret=null", "--qa", "tier>=gold", "--output", out,
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"name":"api"`) || strings.Contains(string(body), `"name":"web"`) {
		t.Fatalf("%s", body)
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"report", "--qn", "a"})
	if err := root.Execute(); err == nil {
		t.Fatal("missing upstream")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "github", "--token", "t", "report"})
	if err := root.Execute(); err == nil {
		t.Fatal("no criteria without tty")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "github", "--token", "t", "report", "--qt", "only", "--scm-search", "q=x"})
	if err := root.Execute(); err == nil {
		t.Fatal("conflict")
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "github", "--token", "t", "report", "--qt", "only"})
	if err := root.Execute(); err == nil {
		t.Fatal("no q fragments")
	}

	existing := filepath.Join(dir, "exists.json")
	if err := os.WriteFile(existing, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srv.URL, "--token", "t", "report", "--qn", "api", "-o", existing})
	if err := root.Execute(); err == nil {
		t.Fatal("exists")
	}

	prof := []byte("root:\n  upstream: gitlab\n  host: " + srv.URL + "\n  token: tok\nreport:\n  qn: api\n  scope: all\n")
	if err := os.WriteFile(filepath.Join(dir, "demo.yaml"), prof, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--profile", "demo", "--retries-number", "0", "report"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--upstream", "gitlab", "--host", srv.URL, "report", "--qn", "api"})
	if err := root.Execute(); err == nil {
		t.Fatal("missing token")
	}
	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "gitlab", "--host", srv.URL, "--token", "t", "--retries-number", "0",
		"report", "--qn", "api", "--force", "--output", existing,
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	norm := NormalizeArgs([]string{"report", "--output", "--force", "-o"})
	if len(norm) != 4 || norm[1] != "--output=" || norm[2] != "--force" || norm[3] != "-o=" {
		t.Fatalf("%v", norm)
	}
}
