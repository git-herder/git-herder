package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/resolved"
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
	if g.Host != resolved.DefaultHost(resolved.UpstreamGitLab) {
		t.Fatalf("host=%q", g.Host)
	}
}

func TestResolveHostScheme(t *testing.T) {
	bare := resolveHostFromProfile(t, "api.github.com")
	if bare != "https://api.github.com" {
		t.Fatalf("bare host=%q", bare)
	}
	httpsHost := resolveHostFromProfile(t, "https://gitlab.example/api/v4")
	if httpsHost != "https://gitlab.example/api/v4" {
		t.Fatalf("https host=%q", httpsHost)
	}
	httpHost := resolveHostFromProfile(t, "http://gitlab.example")
	if httpHost != "http://gitlab.example" {
		t.Fatalf("http host=%q", httpHost)
	}
}

func resolveHostFromProfile(t *testing.T, host string) string {
	t.Helper()
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	cmd := &cobra.Command{Use: "report"}
	root.AddCommand(cmd)
	doc := map[string]any{"root": map[string]any{"upstream": "github", "host": host}}
	g, err := ResolveGlobals(cmd, doc, "report")
	if err != nil {
		t.Fatal(err)
	}
	return g.Host
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
	if g.Host != resolved.DefaultHost(resolved.UpstreamGitHub) {
		t.Fatalf("host=%q", g.Host)
	}
}

func TestResolveGitHTTPSTokenRootOnly(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("verbose", false, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Int("retries-number", 3, "")
	root.PersistentFlags().String("on-error", "continue", "")
	root.PersistentFlags().String("workspace", "/w", "")
	root.PersistentFlags().String("upstream", "", "")
	root.PersistentFlags().String("host", "", "")
	root.PersistentFlags().String("token", "", "")
	root.PersistentFlags().String("git-https-token", "", "")
	cmd := &cobra.Command{Use: "publish"}
	root.AddCommand(cmd)
	doc := map[string]any{"publish": map[string]any{"git-https-token": "from-cmd"}}
	g, err := ResolveGlobals(cmd, doc, "publish")
	if err != nil {
		t.Fatal(err)
	}
	if g.GitHTTPSToken != "" {
		t.Fatalf("command block leaked: %q", g.GitHTTPSToken)
	}
	doc = map[string]any{"root": map[string]any{"git-https-token": "from-root"}}
	g, err = ResolveGlobals(cmd, doc, "publish")
	if err != nil {
		t.Fatal(err)
	}
	if g.GitHTTPSToken != "from-root" {
		t.Fatalf("token=%q", g.GitHTTPSToken)
	}
	if err := root.PersistentFlags().Set("git-https-token", "from-flag"); err != nil {
		t.Fatal(err)
	}
	g, err = ResolveGlobals(cmd, doc, "publish")
	if err != nil {
		t.Fatal(err)
	}
	if g.GitHTTPSToken != "from-flag" {
		t.Fatalf("token=%q", g.GitHTTPSToken)
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
