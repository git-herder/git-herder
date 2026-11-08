package profilestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListIgnoresYmlExtension(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.yml"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("y"), 0o600)
	s := NewAt(dir)
	names, err := s.ListYAMLNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "b" {
		t.Fatalf("names=%v", names)
	}
}
