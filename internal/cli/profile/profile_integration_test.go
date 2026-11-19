package profile_test

import (
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/profilestore"
)

// Сквозной прогон profile через корень CLI и каталог профилей.
func TestProfileIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)

	root, err := cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"profile", "create", "work"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root, err = cli.NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"profile", "use", "work"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	store := profilestore.NewAt(dir)
	ok, err := store.Exists("work")
	if err != nil || !ok {
		t.Fatalf("exists %v err %v", ok, err)
	}
	name, err := store.ReadDefaultName()
	if err != nil || name != "work" {
		t.Fatalf("default %q err %v", name, err)
	}
}
