package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/resolved"
)

func cloneCmd() *cobra.Command {
	root := &cobra.Command{Use: "githerd"}
	cmd := &cobra.Command{Use: "clone", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qp", nil, "")
	fs.StringArray("qn", nil, "")
	fs.StringArray("qt", nil, "")
	fs.StringArray("qa", nil, "")
	fs.StringArray("scm-search", nil, "")
	fs.String("scope", "", "")
	fs.Bool("include-inactive", false, "")
	fs.String("on-limit-exceeded", "", "")
	fs.StringP("existing", "E", "skip", "")
	fs.Bool("preserve-path", false, "")
	fs.BoolP("interactive", "i", false, "")
	fs.StringArray("git-opts", nil, "")
	fs.String("protocol", "ssh", "")
	fs.String("path-attribute", "", "")
	root.AddCommand(cmd)
	return cmd
}

func TestResolveCloneAndGitOpts(t *testing.T) {
	cmd := cloneCmd()
	if err := cmd.ParseFlags([]string{"--qn", "api", "--existing", "drop", "--preserve-path", "--git-opts=--depth=1", "--git-opts=--branch=dev"}); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveClone(cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Existing != ExistingDrop || !got.PreservePath || len(got.GitOpts) != 2 || len(got.Criteria.Names) != 1 {
		t.Fatalf("%+v", got)
	}
	if err := ValidateGitOpts(got.GitOpts); err != nil {
		t.Fatal(err)
	}

	cmd = cloneCmd()
	doc := map[string]any{"clone": map[string]any{"git-opts": []any{"--depth=1", "--filter=blob:none"}}}
	got, err = ResolveClone(cmd, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GitOpts) != 2 || got.Existing != ExistingSkip {
		t.Fatalf("%+v", got)
	}

	cmd = cloneCmd()
	if err := cmd.ParseFlags([]string{"--existing", "SKIP"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveClone(cmd, nil); err == nil {
		t.Fatal("uppercase existing")
	}

	bad := [][]string{
		{"path"},
		{"--"},
		{"--separate-git-dir", "/tmp/x"},
		{"--separate-git-dir=/tmp/x"},
		{"-c"},
		{"-c", "credential.helper=store"},
		{"--config=http.extraHeader=Authorization: x"},
		{"--config=url.https://github.com/.insteadOf=https://x"},
		{"--config=remote.origin.url=https://evil"},
	}
	for _, opts := range bad {
		if err := ValidateGitOpts(opts); err == nil {
			t.Fatalf("accepted %v", opts)
		}
	}
	if err := ValidateGitOpts([]string{"--config=user.name=Ada"}); err != nil {
		t.Fatal(err)
	}
	cmd = cloneCmd()
	if err := cmd.ParseFlags([]string{"--protocol", "https"}); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveClone(cmd, nil)
	if err != nil || got.Protocol != ProtocolHTTPS {
		t.Fatalf("%v %+v", err, got)
	}
	cmd = cloneCmd()
	if err := cmd.ParseFlags([]string{"--protocol", "SSH"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveClone(cmd, nil); err == nil {
		t.Fatal("uppercase protocol")
	}
}

func TestResolvePathAttribute(t *testing.T) {
	cmd := cloneCmd()
	if err := cmd.ParseFlags([]string{"--qn", "api", "--path-attribute", "workspace-path"}); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveClone(cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PathAttributeSet || got.PathAttribute != "workspace-path" {
		t.Fatalf("%+v", got)
	}

	cmd = cloneCmd()
	doc := map[string]any{"clone": map[string]any{"path-attribute": "from-profile"}}
	got, err = ResolveClone(cmd, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PathAttributeSet || got.PathAttribute != "from-profile" {
		t.Fatalf("%+v", got)
	}

	cmd = cloneCmd()
	got, err = ResolveClone(cmd, nil)
	if err != nil || got.PathAttributeSet {
		t.Fatalf("%v %+v", err, got)
	}

	cmd = cloneCmd()
	if err := cmd.ParseFlags([]string{"--path-attribute", "  "}); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveClone(cmd, nil)
	if err != nil || !got.PathAttributeSet || got.PathAttribute != "" {
		t.Fatalf("%v %+v", err, got)
	}
	if err := ValidatePathAttribute(got.PathAttributeSet, got.PathAttribute, resolved.UpstreamGitHub); err == nil {
		t.Fatal("empty name")
	}
	if err := ValidatePathAttribute(true, "workspace-path", resolved.UpstreamGitLab); err == nil {
		t.Fatal("gitlab")
	}
	if err := ValidatePathAttribute(true, "workspace-path", resolved.UpstreamGitHub); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePathAttribute(false, "", resolved.UpstreamGitLab); err != nil {
		t.Fatal(err)
	}
}
