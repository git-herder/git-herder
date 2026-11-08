package profilestore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	if err := ValidateProfileName("ok-name_1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProfileName("default"); err == nil {
		t.Fatal("expected reserved error")
	}
	if err := ValidateProfileName("../x"); err == nil {
		t.Fatal("expected invalid")
	}
}

func TestStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)

	if err := s.CreateEmpty("mine"); err != nil {
		t.Fatal(err)
	}
	ok, err := s.Exists("mine")
	if err != nil || !ok {
		t.Fatal("profile should exist")
	}
	if err := s.WriteDefault("mine"); err != nil {
		t.Fatal(err)
	}
	names, err := s.ListYAMLNames()
	if err != nil || len(names) != 1 {
		t.Fatalf("list: %v %v", names, err)
	}
	b, err := s.ReadYAMLBytes("mine")
	if err != nil || len(b) == 0 {
		t.Fatal(err)
	}
	if err := s.Delete("mine"); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, "default"))
}

func TestLoadForResolveSCMInCommandBlock(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	yaml := "clone:\n  token: x\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := s.LoadForResolve("bad")
	if err == nil {
		t.Fatal("expected config error")
	}
}

func TestBrokenDefault(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	_ = os.WriteFile(filepath.Join(dir, "default"), []byte("missing\n"), 0o600)
	broken, name, err := s.BrokenDefault()
	if err != nil || !broken || name != "missing" {
		t.Fatalf("broken=%v name=%q err=%v", broken, name, err)
	}
}

func TestCopyFrom(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)
	_ = s.CreateEmpty("src")
	if err := s.CopyFrom("dst", "src"); err != nil {
		t.Fatal(err)
	}
	ok, _ := s.Exists("dst")
	if !ok {
		t.Fatal("dst missing")
	}
}
