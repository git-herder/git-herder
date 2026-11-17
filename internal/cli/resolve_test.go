package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/resolved"
)

func TestParseUpstreamResolved(t *testing.T) {
	t.Parallel()
	u, err := resolved.ParseUpstream("github")
	if err != nil || u != resolved.UpstreamGitHub {
		t.Fatalf("%v %v", u, err)
	}
	_, err = resolved.ParseUpstream("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveUpstreamInvalid(t *testing.T) {
	t.Parallel()
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	cmd := &cobra.Command{Use: "x"}
	root.AddCommand(cmd)
	doc := map[string]any{"root": map[string]any{"upstream": "azure"}}
	_, err := ResolveGlobals(cmd, doc, "x")
	if err == nil {
		t.Fatal("expected upstream error")
	}
}

func TestResolveOnErrorInvalid(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	_ = root.PersistentFlags().Set("on-error", "nope")
	cmd := &cobra.Command{Use: "x"}
	root.AddCommand(cmd)
	_, err := ResolveGlobals(cmd, nil, "x")
	if err == nil {
		t.Fatal("expected on-error parse error")
	}
}
