package sync

import (
	"strings"
	"testing"
)

// Сквозной прогон sync: команда выбирает копию и гоняет pull/push через подменённый git.
// API SCM не вызывается. t.Parallel не используется: runner — пакетная переменная.
func TestSyncIntegration(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())

	t.Run("github", func(t *testing.T) {
		runSyncIntegration(t, "github", "https://github.example/acme/api.git", "x-access-token:sekret@github.example")
	})
	t.Run("gitlab", func(t *testing.T) {
		runSyncIntegration(t, "gitlab", "https://gitlab.example/g/api.git", "oauth2:sekret@gitlab.example")
	})
}

func runSyncIntegration(t *testing.T, upstream, remoteURL, tokenUser string) {
	t.Helper()
	git := readyGit()
	git.remoteURL = remoteURL
	git.oidAfterPull = "merged"
	lines := install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	err := runCmd(t,
		"--retries-number", "0", "--git-https-token", "sekret", "--upstream", upstream,
		"--workspace", ws, "sync", "feature", "--git-pull-opts=--ff-only", "--git-push-opts=--no-verify",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ ✔️ api" {
		t.Fatalf("lines %v", *lines)
	}
	if git.pulls != 1 || len(git.lastPullOpts) != 1 || git.lastPullOpts[0] != "--ff-only" {
		t.Fatalf("pulls %d opts %v", git.pulls, git.lastPullOpts)
	}
	if git.pushes != 1 || len(git.lastPushOpts) != 1 || git.lastPushOpts[0] != "--no-verify" {
		t.Fatalf("pushes %d opts %v", git.pushes, git.lastPushOpts)
	}
	if git.lastRemote != "origin" || git.lastBranch != "feature" {
		t.Fatalf("remote %s branch %s", git.lastRemote, git.lastBranch)
	}
	if len(git.cfgs) != 3 {
		t.Fatalf("cfg calls %d", len(git.cfgs))
	}
	for _, cfg := range git.cfgs {
		joined := strings.Join(cfg, " ")
		if !strings.Contains(joined, tokenUser) || !strings.Contains(joined, "insteadOf="+remoteURL) {
			t.Fatalf("cfg %v", cfg)
		}
	}
}
