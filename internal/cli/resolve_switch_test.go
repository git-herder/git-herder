package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func switchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "switch", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "")
	fs.StringArray("qp", nil, "")
	fs.BoolP("interactive", "i", false, "")
	fs.BoolP("all", "A", false, "")
	fs.BoolP("create", "c", false, "")
	fs.BoolP("force-create", "C", false, "")
	fs.Bool("discard-changes", false, "")
	fs.BoolP("merge", "m", false, "")
	fs.String("track", "", "")
	fs.Lookup("track").NoOptDefVal = "origin"
	fs.StringArray("git-opts", nil, "")
	return cmd
}

func TestResolveSwitchArgBeatsProfile(t *testing.T) {
	cmd := switchCmd()
	doc := map[string]any{
		"root":    map[string]any{"branch": "from-root", "start-point": "root-start"},
		"githerd": map[string]any{"branch": "from-parent"},
		"switch": map[string]any{
			"branch":      "from-profile",
			"start-point": "profile-start",
			"qn":          "api",
			"create":      true,
		},
	}
	got, err := ResolveSwitch(cmd, []string{"from-arg", "arg-start"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "from-arg" || !got.StartPointSet || got.StartPoint != "arg-start" || !got.Create {
		t.Fatalf("%+v", got)
	}
	if len(got.QN) != 1 || got.QN[0] != "api" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveSwitchBranchIgnoresRoot(t *testing.T) {
	cmd := switchCmd()
	doc := map[string]any{"root": map[string]any{"branch": "from-root", "track": "upstream"}}
	got, err := ResolveSwitch(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "" || got.StartPointSet || !got.TrackSet || got.Track != "upstream" {
		t.Fatalf("%+v", got)
	}
	doc["switch"] = map[string]any{"branch": "feature", "track": nil, "start-point": ""}
	got, err = ResolveSwitch(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "feature" || !got.TrackSet || got.Track != "origin" || !got.StartPointSet || got.StartPoint != "" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveSwitchBareTrackAndCLIList(t *testing.T) {
	cmd := switchCmd()
	if err := cmd.ParseFlags([]string{"--track", "--git-opts=--quiet"}); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"switch": map[string]any{"git-opts": []any{"--no-quiet"}, "track": "upstream"}}
	got, err := ResolveSwitch(cmd, []string{"feature"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TrackSet || got.Track != "origin" {
		t.Fatalf("%+v", got)
	}
	if len(got.GitOpts) != 1 || got.GitOpts[0] != "--quiet" {
		t.Fatalf("%+v", got)
	}
}

func TestValidateSwitchGitOpts(t *testing.T) {
	if err := ValidateSwitchGitOpts([]string{"--ignore-other-worktrees", "--quiet"}); err != nil {
		t.Fatal(err)
	}
	banned := []string{"-c", "-C", "-m", "-t", "--detach", "--merge=ours", "--force", "--force-with-lease", "--guess", "--", "feature", "--separate-git-dir"}
	for _, tok := range banned {
		if err := ValidateSwitchGitOpts([]string{tok}); err == nil {
			t.Fatalf("expected ban %s", tok)
		}
	}
	if err := ValidateSwitchGitOpts([]string{"-c", "credential.helper=x"}); err == nil {
		t.Fatal("expected banned -c")
	}
	if err := ValidateSwitchGitOpts([]string{"--config=http.extraheader=1"}); err == nil {
		t.Fatal("expected banned config")
	}
	if err := ValidateSwitchGitOpts([]string{"--config=core.autocrlf=true"}); err != nil {
		t.Fatal(err)
	}
}
