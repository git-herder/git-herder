package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cov.out")
	content := "mode: set\npkg/file.go:1.1,2.2 3 2\npkg/file.go:3.1,4.2 1 0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	total, covered, ok := parseCoverProfile(path)
	if !ok || total != 2 || covered != 1 {
		t.Fatalf("total=%d covered=%d ok=%v", total, covered, ok)
	}
}
