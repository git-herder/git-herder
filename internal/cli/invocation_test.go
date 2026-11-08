package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/profilestore"
)

func TestBootstrapRunProfileTree(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	s := profilestore.NewAt(dir)
	_ = s.CreateEmpty("demo")

	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"profile", "demo"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if ActiveRuntime == nil || ActiveRuntime.Trace == nil {
		t.Fatal("expected active trace")
	}
	_ = ActiveRuntime.Trace.Close()
}

func TestBootstrapRejectsProfileFlagOnProfileTree(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)

	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--profile", "x", "profile"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCodeFromError(err) != 1 {
		t.Fatalf("code=%d", ExitCodeFromError(err))
	}
}

func TestResolveBoolFromProfileYAML(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	wd, _ := os.Getwd()
	root.PersistentFlags().String("workspace", wd, "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")

	cmd := &cobra.Command{Use: "clone"}
	root.AddCommand(cmd)
	doc := map[string]any{"root": map[string]any{"debug": true}}
	g, err := ResolveGlobals(cmd, doc, "clone")
	if err != nil {
		t.Fatal(err)
	}
	if !g.Debug {
		t.Fatal("expected debug from profile")
	}
}

func TestShouldSkipBootstrapHelp(t *testing.T) {
	c := &cobra.Command{}
	c.Flags().Bool("help", false, "")
	_ = c.Flags().Set("help", "true")
	if !shouldSkipBootstrap(c) {
		t.Fatal("expected skip when help set")
	}
}

func TestBootstrapRunNonProfileCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.AddCommand(&cobra.Command{
		Use: "noop",
		Run: func(*cobra.Command, []string) {},
	})
	root.SetArgs([]string{"noop"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if ActiveRuntime != nil && ActiveRuntime.Trace != nil {
		_ = ActiveRuntime.Trace.Close()
	}
}

func TestWarnBrokenDefaultIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	_ = os.WriteFile(filepath.Join(dir, "default"), []byte("ghost\n"), 0o600)

	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"profile"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if ActiveRuntime != nil && ActiveRuntime.Trace != nil {
		_ = ActiveRuntime.Trace.Close()
	}
}
