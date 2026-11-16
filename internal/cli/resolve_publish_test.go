package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func publishCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "publish", Run: func(*cobra.Command, []string) {}}
	fs := cmd.Flags()
	fs.StringArray("qp", nil, "")
	fs.StringArray("qn", nil, "")
	fs.String("path-attribute", "", "")
	fs.Bool("leaf-name", false, "")
	fs.String("remote", "origin", "")
	fs.String("existing", "skip", "")
	fs.BoolP("interactive", "i", false, "")
	fs.BoolP("all", "A", false, "")
	fs.String("protocol", "ssh", "")
	return cmd
}

func TestResolvePublishArgBeatsProfile(t *testing.T) {
	cmd := publishCmd()
	doc := map[string]any{
		"root":    map[string]any{"root": "from-global"},
		"publish": map[string]any{"root": "from-publish", "remote": "upstream", "existing": "rename"},
	}
	got, err := ResolvePublish(cmd, []string{"from-arg"}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != "from-arg" || got.Remote != "upstream" || got.Existing != PublishRename || got.Protocol != ProtocolSSH {
		t.Fatalf("%+v", got)
	}
}

func TestResolvePublishRootOnlyFromPublishBlock(t *testing.T) {
	cmd := publishCmd()
	doc := map[string]any{"root": map[string]any{"root": "from-global"}}
	got, err := ResolvePublish(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != "" {
		t.Fatalf("root=%q", got.Root)
	}
	doc["publish"] = map[string]any{"root": "acme/team", "leaf-name": true, "path-attribute": "workspace-path", "qn": "api", "qp": []any{"group"}}
	got, err = ResolvePublish(cmd, nil, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != "acme/team" || !got.LeafName || !got.PathAttributeSet || got.PathAttribute != "workspace-path" {
		t.Fatalf("%+v", got)
	}
	if len(got.QN) != 1 || got.QN[0] != "api" || len(got.QP) != 1 || got.QP[0] != "group" {
		t.Fatalf("filters %+v", got)
	}
}

func TestResolvePublishRejectsExisting(t *testing.T) {
	cmd := publishCmd()
	if err := cmd.ParseFlags([]string{"--existing", "drop", "--protocol", "SSH"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePublish(cmd, []string{"acme"}, nil); err == nil {
		t.Fatal("expected existing error")
	}
	cmd = publishCmd()
	if err := cmd.ParseFlags([]string{"--protocol", "https"}); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePublish(cmd, []string{"acme"}, nil)
	if err != nil || got.Protocol != ProtocolHTTPS {
		t.Fatalf("%+v %v", got, err)
	}
}
