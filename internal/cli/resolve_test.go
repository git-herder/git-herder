package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateUpstream(t *testing.T) {
	if err := ValidateUpstream("github"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstream("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveGlobalsCLIOverridesProfile(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("workspace", "/tmp", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	root.PersistentFlags().Bool("verbose", false, "")
	_ = root.PersistentFlags().Set("debug", "true")

	cmd := &cobra.Command{Use: "report"}
	root.AddCommand(cmd)
	doc := map[string]any{
		"root": map[string]any{"debug": false},
	}
	g, err := ResolveGlobals(cmd, doc, "report")
	if err != nil {
		t.Fatal(err)
	}
	if !g.Debug {
		t.Fatal("CLI debug should win")
	}
}

func TestIsProfileTree(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	p := &cobra.Command{Use: "profile"}
	root.AddCommand(p)
	c := &cobra.Command{Use: "use"}
	p.AddCommand(c)
	if !IsProfileTree(c) {
		t.Fatal("expected profile tree")
	}
	if IsProfileTree(root) {
		t.Fatal("root alone is not profile tree")
	}
}

func TestAbsolutizePathTilde(t *testing.T) {
	p, err := absolutizePath("~/profiles")
	if err != nil {
		t.Fatal(err)
	}
	if p == "" || p == "~/profiles" {
		t.Fatalf("bad path: %q", p)
	}
}
