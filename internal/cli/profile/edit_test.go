package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/profilestore"
)

func TestRunEditNoEditor(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("ed")
	t.Setenv("EDITOR", "")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "edit", "ed"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunEditWithEditor(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("ed")
	var editor string
	if runtime.GOOS == "windows" {
		editor = filepath.Join(dir, "noop.cmd")
		_ = os.WriteFile(editor, []byte("@exit /b 0\r\n"), 0o644)
	} else {
		editor = filepath.Join(dir, "noop.sh")
		_ = os.WriteFile(editor, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	t.Setenv("EDITOR", editor)
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "edit", "ed"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
