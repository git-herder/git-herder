package gitexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("GITHERD_FAKE_GIT") == "1" {
		if os.Getenv("GITHERD_FAKE_GIT_WD") == "1" {
			wd, _ := os.Getwd()
			fmt.Fprintln(os.Stdout, wd)
		}
		if msg := os.Getenv("GITHERD_FAKE_GIT_STDOUT"); msg != "" {
			fmt.Fprintln(os.Stdout, msg)
		}
		fmt.Fprintln(os.Stderr, strings.Join(os.Args[1:], " "))
		if code := os.Getenv("GITHERD_FAKE_GIT_CODE"); code != "" {
			n, _ := strconv.Atoi(code)
			os.Exit(n)
		}
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

func TestParseRemoteVerbose(t *testing.T) {
	in := "origin\tgit@github.com:acme/api.git (fetch)\norigin\tgit@github.com:acme/api.git (push)\nupstream https://gitlab.example/g/api.git (fetch)\n"
	got := parseRemoteVerbose(in)
	if len(got) != 3 || got[0] != "git@github.com:acme/api.git" || got[2] != "https://gitlab.example/g/api.git" {
		t.Fatalf("%v", got)
	}
}

func TestRunInFakeGit(t *testing.T) {
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, name)
	if err := copyFile(src, bin); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHERD_FAKE_GIT", "1")
	t.Setenv("GITHERD_FAKE_GIT_WD", "1")
	work := t.TempDir()
	out, err := runIn(context.Background(), work, []string{"remote", "-v"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "remote -v") || !strings.Contains(out, work) {
		t.Fatalf("out=%q", out)
	}
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "HEAD")
	t.Setenv("GITHERD_FAKE_GIT_WD", "")
	branch, detached, _, err := HeadBranch(context.Background(), work)
	if err != nil || !detached || branch != "" {
		t.Fatalf("branch=%q detached=%v err=%v", branch, detached, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "develop")
	branch, detached, _, err = HeadBranch(context.Background(), work)
	if err != nil || detached || branch != "develop" {
		t.Fatalf("branch=%q detached=%v err=%v", branch, detached, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "")
	t.Setenv("GITHERD_FAKE_GIT_CODE", "1")
	ok, _, err := LocalBranch(context.Background(), work, "main")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "2")
	_, missing, _, err := RemoteURL(context.Background(), work, "origin")
	if err != nil || !missing {
		t.Fatalf("missing=%v err=%v", missing, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "128")
	if _, _, _, err = RemoteURL(context.Background(), work, "origin"); err == nil {
		t.Fatal("expected git error")
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	if _, err = Push(context.Background(), work, "origin", "main"); err != nil {
		t.Fatal(err)
	}
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

func TestCloneMissingGit(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("PATHEXT", "")
	if _, err := Clone(context.Background(), nil, "https://example/r", "dest"); err == nil {
		t.Fatal("expected missing git")
	}
}
