package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSpecURL(t *testing.T) {
	t.Parallel()
	root := &cobra.Command{Use: "githerd"}
	help := &cobra.Command{Use: "help"}
	profile := &cobra.Command{Use: "profile"}
	create := &cobra.Command{Use: "create"}
	remote := &cobra.Command{Use: "remote"}
	root.AddCommand(help, profile, remote)
	profile.AddCommand(create)

	base := specBase
	if specURL(nil) != base+"cli/index.md" || specURL(root) != base+"cli/index.md" || specURL(help) != base+"cli/index.md" {
		t.Fatalf("root/help %s %s %s", specURL(nil), specURL(root), specURL(help))
	}
	if specURL(profile) != base+"cli/profile/index.md" || specURL(create) != base+"cli/profile/index.md" {
		t.Fatalf("profile %s %s", specURL(profile), specURL(create))
	}
	if specURL(remote) != base+"cli/remote/index.md" {
		t.Fatalf("remote %s", specURL(remote))
	}
}

func TestFormatSpecLine(t *testing.T) {
	t.Parallel()
	url := specBase + "cli/remote/index.md"
	if formatSpecLine(url, false) != url {
		t.Fatalf("plain %q", formatSpecLine(url, false))
	}
	want := "\033]8;;" + url + "\033\\index.md\033]8;;\033\\"
	if formatSpecLine(url, true) != want {
		t.Fatalf("link %q", formatSpecLine(url, true))
	}
}
