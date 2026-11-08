package profilestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	got, err := DefaultDir()
	if err != nil || got != dir {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestNewUsesHome(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	s, err := New()
	if err != nil || s == nil {
		t.Fatal(err)
	}
}

func TestParseYAMLAndLoadSuccess(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	yaml := "root:\n  upstream: github\n  token: secret\n"
	if err := os.WriteFile(filepath.Join(dir, "ok.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	name, doc, err := s.LoadForResolve("ok")
	if err != nil || name != "ok" || doc == nil {
		t.Fatalf("name=%q err=%v", name, err)
	}
}

func TestProfilePathAbs(t *testing.T) {
	s := NewAt(t.TempDir())
	p, err := s.ProfilePathAbs("myprof")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Fatal(p)
	}
}

func TestReadYAMLMapInvalid(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	_ = os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":"), 0o600)
	_, err := s.ReadYAMLMap("bad")
	if err == nil {
		t.Fatal("expected yaml error")
	}
}

func TestParseYAMLOK(t *testing.T) {
	s := NewAt(t.TempDir())
	if err := s.ParseYAML([]byte("root: {}\n")); err != nil {
		t.Fatal(err)
	}
}

func TestCopyFileHelper(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(dst, src); err != nil {
		t.Fatal(err)
	}
}
