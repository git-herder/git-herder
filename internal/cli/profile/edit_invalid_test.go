package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/profilestore"
)

func TestRunEditInvalidYAMLAfterEditor(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("bad")
	var editor string
	if runtime.GOOS == "windows" {
		editor = filepath.Join(dir, "w.cmd")
		_ = os.WriteFile(editor, []byte("@echo : > %1\r\n@exit /b 0\r\n"), 0o644)
	} else {
		editor = filepath.Join(dir, "w.sh")
		_ = os.WriteFile(editor, []byte("#!/bin/sh\necho ':' > \"$1\"\n"), 0o755)
	}
	t.Setenv("EDITOR", editor)
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "edit", "bad"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected invalid yaml error")
	}
}

func TestCreateWithEditNoEditor(t *testing.T) {
	dir := t.TempDir()
	s := profilestore.NewAt(dir)
	t.Setenv("EDITOR", "")
	root := &cobra.Command{Use: "githerd"}
	Register(root, s)
	root.SetArgs([]string{"profile", "create", "n", "--edit"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
