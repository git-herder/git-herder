package gitexec

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("GITHERD_FAKE_GIT") == "1" {
		fmt.Fprintln(os.Stderr, strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCloneArgs(t *testing.T) {
	got := cloneArgs([]string{"--depth=1"}, "https://example/r", "dest")
	want := "clone --depth=1 -- https://example/r dest"
	if strings.Join(got, " ") != want {
		t.Fatalf("%v", got)
	}
}

func TestRunBinCapturesOutput(t *testing.T) {
	env := append(os.Environ(), "GITHERD_FAKE_GIT=1")
	out, err := runBin(context.Background(), os.Args[0], []string{"clone", "--", "https://example/r", "dest"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "clone -- https://example/r dest") {
		t.Fatalf("%q", out)
	}
}

func TestCloneMissingGit(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("PATHEXT", "")
	if _, err := Clone(context.Background(), nil, "https://example/r", "dest"); err == nil {
		t.Fatal("expected missing git")
	}
}
