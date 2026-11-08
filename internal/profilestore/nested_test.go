package profilestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedCommandTokenForbidden(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	yaml := "clone:\n  publish:\n    token: x\n"
	_ = os.WriteFile(filepath.Join(dir, "n.yaml"), []byte(yaml), 0o600)
	_, _, err := s.LoadForResolve("n")
	if err == nil {
		t.Fatal("expected nested token error")
	}
}
