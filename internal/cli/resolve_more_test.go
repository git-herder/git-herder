package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveUpstreamHostDefault(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	cmd := &cobra.Command{Use: "clone"}
	root.AddCommand(cmd)
	doc := map[string]any{"root": map[string]any{"upstream": "gitlab"}}
	g, err := ResolveGlobals(cmd, doc, "clone")
	if err != nil {
		t.Fatal(err)
	}
	if g.Host != "gitlab.com" {
		t.Fatalf("host=%q", g.Host)
	}
}

func TestResolveBoolStringTrue(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("debug", false, "")
	flags := root.PersistentFlags()
	doc := map[string]any{"root": map[string]any{"debug": "true"}}
	cmd := &cobra.Command{Use: "x"}
	root.AddCommand(cmd)
	v, err := resolveBoolFlag(flags, "debug", cmd, doc, "x")
	if err != nil || !v {
		t.Fatalf("v=%v err=%v", v, err)
	}
}

func TestResolveUpstreamFromCLI(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	_ = root.PersistentFlags().Set("upstream", "github")
	cmd := &cobra.Command{Use: "clone"}
	root.AddCommand(cmd)
	g, err := ResolveGlobals(cmd, nil, "clone")
	if err != nil {
		t.Fatal(err)
	}
	if g.Host != "github.com" {
		t.Fatalf("host=%q", g.Host)
	}
}

func TestResolveIntFromProfile(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Int("retries-number", 3, "")
	cmd := &cobra.Command{Use: "x"}
	root.AddCommand(cmd)
	doc := map[string]any{"x": map[string]any{"retries-number": float64(9)}}
	v, err := resolveIntFlag(root.PersistentFlags(), "retries-number", cmd, doc, "x")
	if err != nil || v != 9 {
		t.Fatalf("v=%d err=%v", v, err)
	}
}
