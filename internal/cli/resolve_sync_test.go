package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "sync", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "")
	fs.StringArray("qp", nil, "")
	fs.String("remote", "origin", "")
	fs.BoolP("interactive", "i", false, "")
	fs.BoolP("all", "A", false, "")
	fs.StringArray("git-pull-opts", nil, "")
	fs.StringArray("git-push-opts", nil, "")
	return cmd
}

func TestResolveSyncArgBeatsProfile(t *testing.T) {
	cmd := syncCmd()
	doc := map[string]any{
		"root": map[string]any{"branch": "from-root", "remote": "from-root"},
		"sync": map[string]any{"branch": "from-sync", "remote": "upstream", "qn": "api", "qp": []any{"group"}},
	}
	got, err := ResolveSync(cmd, []string{"from-arg"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "from-arg" || got.Remote != "upstream" {
		t.Fatalf("%+v", got)
	}
	if len(got.QN) != 1 || got.QN[0] != "api" || len(got.QP) != 1 || got.QP[0] != "group" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveSyncBranchOnlyFromSyncBlock(t *testing.T) {
	cmd := syncCmd()
	doc := map[string]any{"root": map[string]any{"branch": "from-root", "remote": "up"}}
	got, err := ResolveSync(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "" || got.Remote != "up" {
		t.Fatalf("%+v", got)
	}
	doc["sync"] = map[string]any{
		"branch":        "feature",
		"git-pull-opts": []any{"--ff-only"},
		"git-push-opts": "--no-verify",
		"interactive":   true,
		"all":           false,
	}
	got, err = ResolveSync(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "feature" || !got.Interactive || got.All || got.Remote != "up" {
		t.Fatalf("%+v", got)
	}
	if len(got.PullOpts) != 1 || got.PullOpts[0] != "--ff-only" || len(got.PushOpts) != 1 || got.PushOpts[0] != "--no-verify" {
		t.Fatalf("%+v", got)
	}
}

func TestResolveSyncCLIListReplacesProfile(t *testing.T) {
	cmd := syncCmd()
	if err := cmd.ParseFlags([]string{"--git-pull-opts=--no-verify", "--remote="}); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"sync": map[string]any{"git-pull-opts": []any{"--ff-only"}, "remote": "upstream"}}
	got, err := ResolveSync(cmd, []string{"  "}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "" || got.Remote != "" {
		t.Fatalf("%+v", got)
	}
	if len(got.PullOpts) != 1 || got.PullOpts[0] != "--no-verify" {
		t.Fatalf("%+v", got)
	}
}

func TestValidateSyncGitOpts(t *testing.T) {
	if err := ValidateSyncGitOpts([]string{"--ff-only", "--no-rebase"}, []string{"--no-verify"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSyncGitOpts([]string{"--separate-git-dir"}, nil); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		pull []string
		push []string
	}{
		{[]string{"--rebase"}, nil},
		{[]string{"--rebase=false"}, nil},
		{[]string{"--rebase=interactive"}, nil},
		{[]string{"-r"}, nil},
		{nil, []string{"-f"}},
		{nil, []string{"--force"}},
		{nil, []string{"--force-with-lease"}},
		{nil, []string{"--force-if-includes"}},
		{[]string{"--"}, nil},
		{[]string{"origin"}, nil},
		{[]string{"-c", "credential.helper=store"}, nil},
		{[]string{"-c", "http.extraHeader=X"}, nil},
		{[]string{"--config=url.https://x.insteadof=https://y"}, nil},
		{[]string{"-c=remote.origin.url=https://x"}, nil},
		{[]string{"-c"}, nil},
		{nil, []string{"--config"}},
	}
	for _, tc := range cases {
		if err := ValidateSyncGitOpts(tc.pull, tc.push); err == nil {
			t.Fatalf("accepted pull=%v push=%v", tc.pull, tc.push)
		}
	}
	if err := ValidateSyncGitOpts([]string{"-c", "core.autocrlf=true"}, nil); err != nil {
		t.Fatal(err)
	}
}
