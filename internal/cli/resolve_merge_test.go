package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func mergeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "merge", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "")
	fs.StringArray("qp", nil, "")
	fs.String("remote", "origin", "")
	fs.BoolP("interactive", "i", false, "")
	fs.BoolP("all", "A", false, "")
	fs.BoolP("stay", "s", false, "")
	fs.BoolP("keep-source-branch", "k", false, "")
	fs.Bool("no-fetch", false, "")
	fs.StringArray("git-opts", nil, "")
	fs.StringArray("git-fetch-opts", nil, "")
	fs.StringArray("git-switch-opts", nil, "")
	fs.StringArray("git-merge-opts", nil, "")
	fs.StringArray("git-branch-opts", nil, "")
	return cmd
}

func TestResolveMergeArgBeatsProfile(t *testing.T) {
	cmd := mergeCmd()
	doc := map[string]any{
		"root":  map[string]any{"from": "root-from", "to": "root-to", "remote": "from-root"},
		"merge": map[string]any{"from": "profile-from", "to": "profile-to", "stay": true, "keep-source-branch": true},
	}
	got, err := ResolveMerge(cmd, []string{"arg-from", "arg-to"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "arg-from" || !got.ToSet || got.To != "arg-to" || !got.Stay || !got.KeepSource {
		t.Fatalf("%+v", got)
	}
	if got.Remote != "from-root" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveMergeFromIgnoresRoot(t *testing.T) {
	cmd := mergeCmd()
	doc := map[string]any{"root": map[string]any{"from": "root-from", "to": "root-to"}}
	got, err := ResolveMerge(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "" || got.ToSet || got.Remote != "origin" {
		t.Fatalf("%+v", got)
	}
	doc["merge"] = map[string]any{
		"from":           "feature",
		"git-merge-opts": "--ff-only",
		"git-fetch-opts": []any{"--prune"},
		"remote":         nil,
	}
	got, err = ResolveMerge(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "feature" || got.ToSet || got.Remote != "" {
		t.Fatalf("%+v", got)
	}
	if len(got.MergeOpts) != 1 || got.MergeOpts[0] != "--ff-only" || len(got.FetchOpts) != 1 || got.FetchOpts[0] != "--prune" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveMergeCLIListReplacesProfile(t *testing.T) {
	cmd := mergeCmd()
	if err := cmd.ParseFlags([]string{"--git-merge-opts=--ff-only", "--remote="}); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"merge": map[string]any{"git-merge-opts": []any{"--no-ff"}, "remote": "upstream", "to": "develop"}}
	got, err := ResolveMerge(cmd, []string{"feature"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "feature" || !got.ToSet || got.To != "develop" || got.Remote != "" {
		t.Fatalf("%+v", got)
	}
	if len(got.MergeOpts) != 1 || got.MergeOpts[0] != "--ff-only" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveMergeNoFetch(t *testing.T) {
	cmd := mergeCmd()
	doc := map[string]any{"merge": map[string]any{"no-fetch": true, "from": "feature"}}
	got, err := ResolveMerge(cmd, []string{"feature"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NoFetch {
		t.Fatalf("%+v", got)
	}
	if err := cmd.ParseFlags([]string{"--no-fetch=false"}); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveMerge(cmd, []string{"feature"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.NoFetch {
		t.Fatalf("%+v", got)
	}
}

func TestValidateMergeGitOpts(t *testing.T) {
	if err := ValidateMergeGitOpts([]string{"--quiet"}, []string{"--prune"}, nil, []string{"--ff-only"}, nil); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"--abort", "--continue=1", "--quit", "--edit", "--separate-git-dir", "feature"} {
		if err := ValidateMergeGitOpts(nil, nil, []string{tok}, nil, nil); err == nil {
			t.Fatalf("expected ban %s", tok)
		}
	}
	if err := ValidateMergeGitOpts(nil, []string{"--config=url.x.insteadof=y"}, nil, nil, nil); err == nil {
		t.Fatal("expected banned config")
	}
}
