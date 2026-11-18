package proxy

import (
	"path/filepath"
	"testing"

	"github.com/git-herder/git-herder/internal/gitexec"
)

func TestFormatAndReason(t *testing.T) {
	t.Parallel()
	if got := formatResult(true, "a/b", ""); got != "✔️ a/b" {
		t.Fatalf("%q", got)
	}
	if got := formatResult(false, "a", "boom"); got != "❌ a\n    boom" {
		t.Fatalf("%q", got)
	}
	if got := shortReason(gitexec.Result{Stdout: "out\n"}); got != "out" {
		t.Fatalf("%q", got)
	}
	if got := shortReason(gitexec.Result{}); got != "git failed" {
		t.Fatalf("%q", got)
	}
	if got := firstNonEmpty(" \n\tok"); got != "ok" {
		t.Fatalf("%q", got)
	}
	root := t.TempDir()
	rel, err := relToWorkspace(root, root)
	if err != nil || rel != "." {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
	child := filepath.Join(root, "documentation", "architecture")
	rel, err = relToWorkspace(root, child)
	if err != nil || rel != "documentation/architecture" {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
}

func TestStoredURLAndNames(t *testing.T) {
	t.Parallel()
	if got := storedURL("https://example/a.git\r\n"); got != "https://example/a.git" {
		t.Fatalf("%q", got)
	}
	names := nonEmptyLines(" origin \n\nupstream\n")
	if len(names) != 2 || names[0] != "origin" || names[1] != "upstream" {
		t.Fatalf("%v", names)
	}
}
