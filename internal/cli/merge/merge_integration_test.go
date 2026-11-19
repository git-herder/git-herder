package merge

import (
	"strings"
	"testing"
)

// Сквозной прогон merge: fetch с https-токеном, switch и merge одной копии.
// t.Parallel не используется: runner — пакетная переменная.
func TestMergeIntegration(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())

	t.Run("github", func(t *testing.T) {
		runMergeIntegration(t, "github", "https://github.example/acme/api.git", "x-access-token:sekret@github.example")
	})
	t.Run("gitlab", func(t *testing.T) {
		runMergeIntegration(t, "gitlab", "https://gitlab.example/g/api.git", "oauth2:sekret@gitlab.example")
	})
}

func runMergeIntegration(t *testing.T, upstream, remoteURL, tokenUser string) {
	t.Helper()
	git := &fakeGit{
		entryBranch:    "feature",
		entryOID:       "old",
		afterBranch:    "main",
		oidAfterSwitch: "aaa",
		oidAfterMerge:  "bbb",
		remoteURL:      remoteURL,
	}
	lines := install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	err := runCmd(t,
		"--git-https-token", "sekret", "--upstream", upstream,
		"--workspace", ws, "merge", "feature", "main",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ ✔️ ✔️ ✔️ ➡️ api" {
		t.Fatalf("lines %v", *lines)
	}
	if git.fetches != 1 || git.lastRemote != "origin" {
		t.Fatalf("fetches %d remote %s", git.fetches, git.lastRemote)
	}
	if len(git.switchReqs) != 1 || git.switchReqs[0].Target != "main" {
		t.Fatalf("switch %+v", git.switchReqs)
	}
	joined := strings.Join(git.lastCfg, " ")
	if !strings.Contains(joined, tokenUser) || !strings.Contains(joined, "insteadOf="+remoteURL) {
		t.Fatalf("cfg %v", git.lastCfg)
	}
}
