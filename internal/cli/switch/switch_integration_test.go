package switchcmd

import "testing"

// Сквозной прогон switch: одна копия переключается на существующую ветку.
// t.Parallel не используется: runner — пакетная переменная.
func TestSwitchIntegration(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	git := &fakeGit{symName: "main", symAfter: "feature", oid: "aaa", oidAfter: "bbb"}
	lines := install(t, git)
	ws := t.TempDir()
	makeCopy(t, ws, "api")
	if err := runCmd(t, "--workspace", ws, "switch", "feature"); err != nil {
		t.Fatal(err)
	}
	if len(*lines) != 1 || (*lines)[0] != "✔️ ➡️ api" {
		t.Fatalf("lines %v", *lines)
	}
	if git.switches != 1 || git.lastReq.Target != "feature" {
		t.Fatalf("switches %d req %+v", git.switches, git.lastReq)
	}
	if len(git.verified) != 1 || git.verified[0] != "refs/heads/feature" {
		t.Fatalf("verified %v", git.verified)
	}
}
