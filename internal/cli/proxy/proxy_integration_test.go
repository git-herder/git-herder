package proxy_test

import (
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/gitexec"
)

// Сквозной прогон pull, push, fetch и remote. Раннер общий, поэтому тесты
// живут в одном пакете и не идут параллельно.
func TestPullIntegration(t *testing.T)   { runProxyIntegration(t, "pull") }
func TestPushIntegration(t *testing.T)   { runProxyIntegration(t, "push") }
func TestFetchIntegration(t *testing.T)  { runProxyIntegration(t, "fetch") }
func TestRemoteIntegration(t *testing.T) { runProxyIntegration(t, "remote") }

func runProxyIntegration(t *testing.T, verb string) {
	t.Helper()
	t.Run("github", func(t *testing.T) {
		runProxyOne(t, verb, "github", "https://github.example/acme/api.git", "x-access-token:sekret@github.example")
	})
	t.Run("gitlab", func(t *testing.T) {
		runProxyOne(t, verb, "gitlab", "https://gitlab.example/g/api.git", "oauth2:sekret@gitlab.example")
	})
}

func runProxyOne(t *testing.T, verb, upstream, remoteURL, tokenUser string) {
	t.Helper()
	ws, git, lines := setup(t)
	makeCopy(t, ws, "api")
	git.fn = func(args []string) (gitexec.Result, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "remote":
			return gitexec.Result{Stdout: "origin\n"}, nil
		case strings.HasPrefix(joined, "remote get-url"):
			return gitexec.Result{Stdout: remoteURL + "\n"}, nil
		default:
			return gitexec.Result{}, nil
		}
	}
	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{
		"--workspace", ws, "--git-https-token", "sekret", "--upstream", upstream, verb,
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ api" {
		t.Fatalf("lines %q", *lines)
	}
	got := verbWithToken(git.calls, verb)
	if len(got) != 1 {
		t.Fatalf("verb calls %d all %v", len(got), git.calls)
	}
	joined := strings.Join(got[0].args, " ")
	if !strings.Contains(joined, tokenUser) || !strings.Contains(joined, "insteadOf="+remoteURL) {
		t.Fatalf("args %s", joined)
	}
}

func verbWithToken(calls []call, verb string) []call {
	var out []call
	for _, c := range calls {
		joined := strings.Join(c.args, " ")
		if commandWord(c.args) == verb && strings.Contains(joined, "insteadOf=") {
			out = append(out, c)
		}
	}
	return out
}

func commandWord(args []string) string {
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
		return a
	}
	return ""
}
