package cli

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/resolved"
)

func reportCmd() *cobra.Command {
	root := &cobra.Command{Use: "githerd"}
	cmd := &cobra.Command{Use: "report", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qp", nil, "")
	fs.StringArray("qn", nil, "")
	fs.StringArray("qt", nil, "")
	fs.StringArray("qa", nil, "")
	fs.StringArray("scm-search", nil, "")
	fs.String("scope", "", "")
	fs.Bool("include-inactive", false, "")
	fs.String("on-limit-exceeded", "", "")
	fs.Bool("force", false, "")
	fs.StringP("output", "o", "", "")
	root.AddCommand(cmd)
	return cmd
}

func TestResolveReportFromArgvAndProfile(t *testing.T) {
	cmd := reportCmd()
	if err := cmd.ParseFlags([]string{"--qp", "group", "--qt", "foo=null", "--qa", "tier>=gold", "--scope", "all", "--force", "--output="}); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveReport(cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.JSONDefault || !got.Force || got.Criteria.Scope != resolved.SelectionScopeAll {
		t.Fatalf("%+v", got)
	}
	if len(got.Criteria.Tags) != 1 || !got.Criteria.Tags[0].Absent {
		t.Fatal(got.Criteria.Tags)
	}

	cmd = reportCmd()
	doc := map[string]any{
		"report": map[string]any{
			"qn":                []any{"api", "web"},
			"scm-search":        "q=org:acme",
			"include-inactive":  true,
			"on-limit-exceeded": "abort",
			"output":            "out.json",
			"force":             true,
		},
	}
	got, err = ResolveReport(cmd, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Criteria.Search || len(got.Criteria.Names) != 2 || !got.Criteria.IncludeInactiveSet || got.Criteria.OnLimit != "abort" {
		t.Fatalf("%+v", got.Criteria)
	}
	if !got.Force || got.JSONPath == "" {
		t.Fatal(got)
	}

	cmd = reportCmd()
	if err := cmd.ParseFlags([]string{"--scm-search", "q=a", "--scm-search", "q=b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveReport(cmd, nil); err == nil {
		t.Fatal("repeat")
	}
	cmd = reportCmd()
	if err := cmd.ParseFlags([]string{"--qt", "bad=value"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveReport(cmd, nil); err == nil {
		t.Fatal("qt")
	}
	cmd = reportCmd()
	if err := cmd.ParseFlags([]string{"--scope", "nope"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveReport(cmd, nil); err == nil {
		t.Fatal("scope")
	}
	cmd = reportCmd()
	got, err = ResolveReport(cmd, map[string]any{"report": map[string]any{"output": ""}})
	if err != nil || !got.JSONDefault || got.Stdout {
		t.Fatal(err, got)
	}
	cmd = reportCmd()
	if _, err := ResolveReport(cmd, map[string]any{"report": map[string]any{"qn": 1}}); err == nil {
		t.Fatal("bad list")
	}
	abs := filepath.Join(t.TempDir(), "r.json")
	cmd = reportCmd()
	if err := cmd.ParseFlags([]string{"--output", abs}); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveReport(cmd, nil)
	if err != nil || got.JSONPath != abs {
		t.Fatalf("%+v %v", got, err)
	}
}
