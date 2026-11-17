package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/redact"
)

func TestInsteadOfRewritesThisProcessOnly(t *testing.T) {
	auth := HTTPSAuth{Token: "sekret", Upstream: "github"}
	args, err := auth.InsteadOf("https://github.com/acme/api.git")
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "-c" {
		t.Fatalf("%v", args)
	}
	if !strings.Contains(args[1], "url.https://x-access-token:sekret@github.com/acme/api.git.insteadOf=https://github.com/acme/api.git") {
		t.Fatalf("%s", args[1])
	}
	logged := redact.String(args[1])
	if strings.Contains(logged, "sekret") {
		t.Fatalf("token leaked: %s", logged)
	}

	gl, err := (HTTPSAuth{Token: "sekret", Upstream: "gitlab"}).InsteadOf("https://gitlab.com/acme/api.git")
	if err != nil || !strings.Contains(gl[1], "oauth2:sekret@") {
		t.Fatalf("%v %v", gl, err)
	}
	emptyUp, err := (HTTPSAuth{Token: "sekret"}).InsteadOf("git@github.com:acme/api.git")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyUp[1], "url.https://x-access-token:sekret@github.com/acme/api.git.insteadOf=git@github.com:acme/api.git") {
		t.Fatalf("%s", emptyUp[1])
	}
	ssh, err := (HTTPSAuth{Token: "sekret", Upstream: "github"}).InsteadOf("ssh://git@github.com/acme/api.git")
	if err != nil || !strings.Contains(ssh[1], "insteadOf=ssh://git@github.com/acme/api.git") || !strings.Contains(ssh[1], "https://x-access-token:sekret@github.com/acme/api.git") {
		t.Fatalf("%v %v", ssh, err)
	}
	if _, err := (HTTPSAuth{Token: ""}).InsteadOf("git@github.com:acme/api.git"); err != nil {
		t.Fatal(err)
	}
	none, err := (HTTPSAuth{Token: "  "}).InsteadOf("git@github.com:acme/api.git")
	if err != nil || none != nil {
		t.Fatalf("%v %v", none, err)
	}
	if _, err := (HTTPSAuth{Token: "sekret"}).InsteadOf("file:///tmp/repo.git"); err == nil {
		t.Fatal("expected unsupported url error")
	}
	httpArgs, err := (HTTPSAuth{Token: "sekret"}).InsteadOf("http://github.com/acme/api.git")
	if err != nil || !strings.Contains(httpArgs[1], "https://x-access-token:sekret@github.com/acme/api.git.insteadOf=http://github.com/acme/api.git") {
		t.Fatalf("%v %v", httpArgs, err)
	}
}

func TestExchangeFakeGit(t *testing.T) {
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHERD_FAKE_GIT", "1")
	work := t.TempDir()
	ctx := context.Background()

	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "origin\n\nupstream")
	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	names, _, err := RemoteNames(ctx, work)
	if err != nil || len(names) != 2 || names[0] != "origin" || names[1] != "upstream" {
		t.Fatalf("%v %v", names, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "abc123")
	oid, _, err := HeadOID(ctx, work)
	if err != nil || oid != "abc123" {
		t.Fatalf("%q %v", oid, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "zzz\trefs/heads/other\nabc\trefs/heads/feature")
	got, found, res, err := LsRemoteHead(ctx, work, "origin", "feature", []string{"-c", "k=v"})
	if err != nil || !found || got != "abc" {
		t.Fatalf("oid=%q found=%v err=%v", got, found, err)
	}
	if !strings.Contains(res.Stderr, "-c k=v ls-remote --heads origin refs/heads/feature") {
		t.Fatalf("%q", res.Stderr)
	}
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "zzz\trefs/heads/other")
	_, found, _, err = LsRemoteHead(ctx, work, "origin", "feature", nil)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "128")
	_, _, res, err = LsRemoteHead(ctx, work, "origin", "feature", nil)
	if err == nil || res.Exit != 128 {
		t.Fatalf("exit=%d err=%v", res.Exit, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "")
	res, err = Pull(ctx, work, []string{"--ff-only"}, "origin", "feature", []string{"-c", "url.https://x.insteadOf=https://h/r"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stderr, "-c url.https://x.insteadOf=https://h/r pull --no-rebase --ff --ff-only -- origin feature") {
		t.Fatalf("%q", res.Stderr)
	}
	res, err = PushOpts(ctx, work, []string{"--no-verify"}, "origin", "feature", nil)
	if err != nil || !strings.Contains(res.Stderr, "push --no-verify -- origin feature") {
		t.Fatalf("%q %v", res.Stderr, err)
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "1")
	ok, _, err := MergeHead(ctx, work)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	ok, _, err = MergeHead(ctx, work)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	t.Setenv("GITHERD_FAKE_GIT_CODE", "128")
	if _, _, err = MergeHead(ctx, work); err == nil {
		t.Fatal("expected merge error")
	}

	t.Setenv("GITHERD_FAKE_GIT_CODE", "0")
	t.Setenv("GITHERD_FAKE_GIT_STDOUT", "foo.md\nbar.go")
	names, _, err = Unmerged(ctx, work)
	if err != nil || len(names) != 2 || names[0] != "foo.md" || names[1] != "bar.go" {
		t.Fatalf("%v %v", names, err)
	}
}

func TestExchangeMissingGit(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("PATHEXT", "")
	res, err := Pull(context.Background(), t.TempDir(), nil, "origin", "main", nil)
	if err == nil || res.Exit == 128 {
		t.Fatalf("exit=%d err=%v", res.Exit, err)
	}
}
