package bootstrap

import (
	"testing"

	"github.com/git-herder/git-herder/internal/profilestore"
)

func TestRootForTestProfileList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	_ = profilestore.NewAt(dir).CreateEmpty("t1")

	root, err := RootForTest()
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestRunWithArgsProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	code := RunWithArgs([]string{"profile"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunWithArgsVersion(t *testing.T) {
	code := RunWithArgs([]string{"--version"})
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunWithArgsVerboseDebug(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	if code := RunWithArgs([]string{"-V", "-D", "profile"}); code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunCallsExitHook(t *testing.T) {
	var code int
	old := exitFn
	exitFn = func(c int) { code = c }
	defer func() { exitFn = old }()
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	Run()
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunWithArgsProfileFlagForbidden(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	if code := RunWithArgs([]string{"--profile", "x", "profile"}); code != 1 {
		t.Fatalf("code=%d", code)
	}
}
