package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatConflictNotes(t *testing.T) {
	if got := conflictNote(nil); got != "" {
		t.Fatal(got)
	}
	if got := conflictNote([]string{"a", "b", "c"}); got != "⚔️ a, b, c" {
		t.Fatalf("%q", got)
	}
	if got := conflictNote([]string{"a", "b", "c", "d"}); got != "⚔️ a, b, c and 1 more files" {
		t.Fatalf("%q", got)
	}
	line := formatResult(signFail, signFail, "glossary", []string{"pull failed", "push did not start"})
	want := "❌ ❌ glossary\n    pull failed\n    push did not start"
	if line != want {
		t.Fatalf("%q", line)
	}
}

func TestRelToWorkspace(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "documentation", "architecture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := relToWorkspace(root, dir)
	if err != nil || got != "documentation/architecture" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = relToWorkspace(root, root)
	if err != nil || got != "." {
		t.Fatalf("%q %v", got, err)
	}
}
