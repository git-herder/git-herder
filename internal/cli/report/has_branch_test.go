package report

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/appexit"
	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/scm/selection"
)

func TestReportHasBranchContinueWritesThenExits(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/branches/") {
			if strings.Contains(r.URL.Path, "/projects/2/") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"name":"main"}`))
			return
		}
		w.Header().Set("X-Total-Pages", "1")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"keep","path_with_namespace":"g/keep","web_url":"https://gitlab.example/g/keep"},
			{"id":2,"name":"bad","path_with_namespace":"g/bad","web_url":"https://gitlab.example/g/bad"}
		]`))
	}))
	defer srv.Close()

	out := filepath.Join(dir, "out.json")
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "gitlab", "--host", srv.URL, "--token", "tok", "--retries-number", "0",
		"--on-error", "continue",
		"report", "--has-branch", "main", "--output", out,
	})
	err = root.Execute()
	var coded appexit.CodedError
	if !errors.As(err, &coded) || coded.Code != 1 || !errors.Is(err, selection.ErrObjectFailed) {
		t.Fatalf("%v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `"name":"keep"`) || strings.Contains(text, `"name":"bad"`) || !strings.Contains(text, `"branches":["main"]`) {
		t.Fatalf("%s", text)
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--upstream", "gitlab", "--host", srv.URL, "--token", "tok",
		"report", "--has-branch", " ",
	})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --has-branch") {
		t.Fatalf("%v", err)
	}
}
