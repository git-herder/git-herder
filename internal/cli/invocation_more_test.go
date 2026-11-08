package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestBootstrapMissingProfileOnOtherCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.AddCommand(&cobra.Command{
		Use: "noop",
		Run: func(*cobra.Command, []string) {},
	})
	root.SetArgs([]string{"--profile", "ghost", "noop"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected missing profile error")
	}
}

func TestBootstrapInvalidUpstream(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.AddCommand(&cobra.Command{
		Use: "noop",
		Run: func(*cobra.Command, []string) {},
	})
	root.SetArgs([]string{"--upstream", "azure", "noop"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected upstream error")
	}
}

func TestExitCodeUnknownSubcommand(t *testing.T) {
	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"nope"})
	err = root.Execute()
	if err == nil {
		t.Fatal()
	}
	if ExitCodeFromError(err) != 2 {
		t.Fatalf("code=%d", ExitCodeFromError(err))
	}
}

func TestLookupProfileParentChain(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	parent := &cobra.Command{Use: "publish"}
	root.AddCommand(parent)
	doc := map[string]any{"publish": map[string]any{"on-error": "abort"}}
	v, ok := lookupProfileValue(doc, parent, "publish", "on-error")
	if !ok || v.(string) != "abort" {
		t.Fatalf("v=%v ok=%v", v, ok)
	}
}
