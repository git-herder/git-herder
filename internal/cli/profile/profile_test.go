package profile

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/profilestore"
)

func TestProfileListAndShow(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("alpha")

	root := &cobra.Command{Use: "githerd"}
	Register(root, s)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root.SetArgs([]string{"profile"})
	err := root.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("alpha")) {
		t.Fatalf("list output: %q", buf.String())
	}

	root2 := &cobra.Command{Use: "githerd"}
	Register(root2, s)
	root2.SetArgs([]string{"profile", "alpha"})
	outBuf := captureStdout(func() {
		_ = root2.Execute()
	})
	if !bytes.Contains(outBuf, []byte("root")) {
		t.Fatalf("show: %q", outBuf)
	}
}

func TestProfileCreateFrom(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("src")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "create", "dst", "--from", "src"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	ok, _ := s.Exists("dst")
	if !ok {
		t.Fatal("dst missing")
	}
}

func TestProfileCreateAndUse(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "create", "newprof"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"profile", "use", "newprof"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	name, _ := s.ReadDefaultName()
	if name != "newprof" {
		t.Fatalf("default=%q", name)
	}
}

func TestProfileDropDefaultForbidden(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("only")
	_ = s.WriteDefault("only")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "drop", "only"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func captureStdout(fn func()) []byte {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.Bytes()
}

func TestProfileDropDeletes(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("gone")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "drop", "gone"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	ok, _ := s.Exists("gone")
	if ok {
		t.Fatal("should be deleted")
	}
}

func TestCreateFromMissing(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "create", "x", "--from", "missing"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("dup")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "create", "dup"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestShowNotFound(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "missing"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProfilePathViaStore(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	p, err := s.ProfilePathAbs("x")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "x.yaml" {
		t.Fatal(p)
	}
}
