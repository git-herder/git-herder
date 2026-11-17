package sync

import (
	"context"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/resolved"
)

func runOne(t *testing.T, git *fakeGit, retries int, onError resolved.OnError, auth gitexec.HTTPSAuth, pullOpts, pushOpts []string) ([]string, bool, error) {
	t.Helper()
	var lines []string
	job := &syncJob{dir: "copy", rel: "documentation/architecture"}
	bad, err := syncCopies(context.Background(), []*syncJob{job}, "feature", "origin", pullOpts, pushOpts, auth, retries, onError, git, func(string) {}, func(s string) {
		lines = append(lines, s)
	})
	return lines, bad, err
}

func TestSyncBothSidesChanged(t *testing.T) {
	git := readyGit()
	git.oidAfterPull = "merge"
	git.lsOID = "remote"
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, []string{"--ff-only"}, []string{"--no-verify"})
	if err != nil || bad || git.pulls != 1 || git.pushes != 1 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	if len(lines) != 1 || lines[0] != "✔️ ✔️ documentation/architecture" {
		t.Fatalf("%q", lines)
	}
	if len(git.lastPullOpts) != 1 || git.lastPullOpts[0] != "--ff-only" || git.lastRemote != "origin" {
		t.Fatalf("opts %v remote %s", git.lastPullOpts, git.lastRemote)
	}
}

func TestSyncNothingToExchange(t *testing.T) {
	git := readyGit()
	git.oid = "aaa"
	git.lsOID = "aaa"
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || bad || len(lines) != 1 || lines[0] != "➡️ ➡️ documentation/architecture" {
		t.Fatalf("%v %v %q", bad, err, lines)
	}
}

func TestSyncMissingRemoteBranchCreatesIt(t *testing.T) {
	git := readyGit()
	git.lsFound = false
	git.oid = "local"
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || bad || git.pulls != 0 || git.pushes != 1 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	if lines[0] != "➡️ ✔️ documentation/architecture" {
		t.Fatalf("%q", lines)
	}
}

func TestSyncMissingRemoteName(t *testing.T) {
	git := readyGit()
	git.names = nil
	lines, bad, err := runOne(t, git, 3, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.lses != 0 {
		t.Fatalf("bad=%v err=%v lses=%d", bad, err, git.lses)
	}
	want := "❌ ❌ documentation/architecture\n    ремота нет\n    отправка не начиналась"
	if lines[0] != want {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncRemoteListError(t *testing.T) {
	git := readyGit()
	git.namesErr = errString("remote failed")
	lines, bad, err := runOne(t, git, 2, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.lses != 0 || !strings.Contains(lines[0], "names failed") {
		t.Fatalf("%v %v %q", bad, err, lines)
	}
}

func TestSyncPullRejectedNoRetry(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	git.pullStderr = "\n\nCould not read from remote repository.\nmore"
	git.pullStdout = "ignored stdout"
	lines, bad, err := runOne(t, git, 4, resolved.OnErrorAbort, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pulls != 1 || git.pushes != 0 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	want := "❌ ❌ documentation/architecture\n    Could not read from remote repository.\n    отправка не начиналась"
	if lines[0] != want {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncPullEmptyOutput(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	lines, _, _ := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if !strings.Contains(lines[0], "получение не удалось") {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncConflictThenPush(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	git.merge = true
	git.unmerged = []string{"foo.md", "bar.go", "buzz.txt", "a", "b", "c", "d", "e"}
	git.oid = "local"
	git.lsOID = "remote"
	lines, bad, err := runOne(t, git, 3, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pulls != 1 || git.pushes != 1 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	want := "⚔️ ✔️ documentation/architecture\n    ⚔️ foo.md, bar.go, buzz.txt and 5 more files"
	if lines[0] != want {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncConflictWithoutNames(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	git.merge = true
	git.unmergedErr = errString("diff failed")
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pushes != 1 {
		t.Fatalf("%v %v pushes=%d", bad, err, git.pushes)
	}
	if strings.Contains(lines[0], "\n") {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncConflictPushNetworkDoesNotRetry(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	git.merge = true
	git.unmerged = []string{"foo.md"}
	git.pushExit = 128
	git.pushStderr = "connection reset"
	lines, bad, err := runOne(t, git, 5, resolved.OnErrorAbort, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pushes != 1 || git.pulls != 1 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	if !strings.Contains(lines[0], "⚔️ ❌") || !strings.Contains(lines[0], "connection reset") {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncPushRejectedNoRetry(t *testing.T) {
	git := readyGit()
	git.oid = "aaa"
	git.lsOID = "aaa"
	git.pushExit = 1
	git.pushStderr = "failed to push some refs"
	lines, bad, err := runOne(t, git, 4, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pushes != 1 {
		t.Fatalf("bad=%v err=%v pushes=%d", bad, err, git.pushes)
	}
	if !strings.HasPrefix(lines[0], "➡️ ❌") || !strings.Contains(lines[0], "failed to push some refs") {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncNetworkRetriesOnce(t *testing.T) {
	git := readyGit()
	git.lsSeq = []int{128, 128}
	git.lsStderr = "Could not resolve host"
	lines, bad, err := runOne(t, git, 1, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.lses != 2 || git.pulls != 0 || len(lines) != 1 {
		t.Fatalf("bad=%v err=%v lses=%d lines=%q", bad, err, git.lses, lines)
	}
	if !strings.Contains(lines[0], "Could not resolve host") || !strings.Contains(lines[0], "отправка не начиналась") {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncNetworkThenSuccess(t *testing.T) {
	git := readyGit()
	git.lsSeq = []int{128, 0}
	git.oid = "aaa"
	git.lsOID = "aaa"
	lines, bad, err := runOne(t, git, 1, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || bad || git.lses != 2 || git.pulls != 1 || len(lines) != 1 {
		t.Fatalf("bad=%v err=%v lses=%d pulls=%d lines=%q", bad, err, git.lses, git.pulls, lines)
	}
	if lines[0] != "➡️ ➡️ documentation/architecture" {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncAbortSkipsNextCopy(t *testing.T) {
	git := readyGit()
	git.failDir = "a"
	var lines []string
	jobs := []*syncJob{{dir: "a", rel: "a"}, {dir: "b", rel: "b"}}
	bad, err := syncCopies(context.Background(), jobs, "feature", "origin", nil, nil, gitexec.HTTPSAuth{}, 0, resolved.OnErrorAbort, git, func(string) {}, func(s string) {
		lines = append(lines, s)
	})
	if err == nil || !bad || git.lses != 1 || len(lines) != 1 {
		t.Fatalf("bad=%v err=%v lses=%d lines=%d", bad, err, git.lses, len(lines))
	}
}

func TestSyncLsOtherCodeNoRetry(t *testing.T) {
	git := readyGit()
	git.lsExit = 1
	git.lsStderr = "local failure"
	_, bad, err := runOne(t, git, 5, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.lses != 1 || git.pulls != 0 {
		t.Fatalf("bad=%v err=%v lses=%d", bad, err, git.lses)
	}
}

func TestSyncPullNetwork(t *testing.T) {
	git := readyGit()
	git.pullExit = 128
	git.pullStderr = "timed out"
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorAbort, gitexec.HTTPSAuth{}, nil, nil)
	if err == nil || !bad || git.pushes != 0 || !strings.Contains(lines[0], "timed out") {
		t.Fatalf("bad=%v err=%v %q", bad, err, lines)
	}
}

func TestSyncTokenRewritesURLButKeepsRemoteName(t *testing.T) {
	git := readyGit()
	git.remoteURL = "https://github.com/acme/api.git"
	git.oid = "aaa"
	git.lsOID = "aaa"
	auth := gitexec.HTTPSAuth{Token: "sekret", Upstream: "github"}
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, auth, nil, nil)
	if err != nil || bad {
		t.Fatal(err)
	}
	if git.lastRemote != "origin" {
		t.Fatalf("remote %s", git.lastRemote)
	}
	joined := strings.Join(git.lastCfg, " ")
	if !strings.Contains(joined, "insteadOf=https://github.com/acme/api.git") || !strings.Contains(joined, "x-access-token:sekret@") {
		t.Fatalf("%s", joined)
	}
	if lines[0] != "➡️ ➡️ documentation/architecture" {
		t.Fatalf("%q", lines[0])
	}
}

func TestSyncTokenSSHRemote(t *testing.T) {
	git := readyGit()
	git.remoteURL = "git@github.com:acme/api.git"
	auth := gitexec.HTTPSAuth{Token: "sekret", Upstream: "gitlab"}
	_, _, err := runOne(t, git, 0, resolved.OnErrorContinue, auth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(git.lastCfg, " ")
	if !strings.Contains(joined, "oauth2:sekret@github.com/acme/api.git.insteadOf=git@github.com:acme/api.git") {
		t.Fatalf("%s", joined)
	}
}

func TestSyncTokenUnsupportedURL(t *testing.T) {
	git := readyGit()
	git.remoteURL = "file:///tmp/repo.git"
	auth := gitexec.HTTPSAuth{Token: "sekret"}
	lines, bad, err := runOne(t, git, 2, resolved.OnErrorContinue, auth, nil, nil)
	if err != nil || !bad || git.lses != 0 || !strings.Contains(lines[0], "cannot take an https token") {
		t.Fatalf("bad=%v err=%v lses=%d %q", bad, err, git.lses, lines)
	}
}

func TestSyncHeadOIDErrors(t *testing.T) {
	git := readyGit()
	git.oidErrOn = 1
	_, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pulls != 0 {
		t.Fatalf("before bad=%v err=%v pulls=%d", bad, err, git.pulls)
	}
	git = readyGit()
	git.oidErrOn = 2
	_, bad, err = runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pulls != 1 || git.pushes != 0 {
		t.Fatalf("after bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
	git = readyGit()
	git.oidErrOn = 3
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pushes != 1 || !strings.Contains(lines[0], "no head") {
		t.Fatalf("push oid bad=%v err=%v %q", bad, err, lines)
	}
}

func TestSyncMergeCheckError(t *testing.T) {
	git := readyGit()
	git.pullExit = 1
	git.mergeErr = errString("merge check failed")
	_, bad, err := runOne(t, git, 3, resolved.OnErrorContinue, gitexec.HTTPSAuth{}, nil, nil)
	if err != nil || !bad || git.pulls != 1 || git.pushes != 0 {
		t.Fatalf("bad=%v err=%v pulls=%d pushes=%d", bad, err, git.pulls, git.pushes)
	}
}

func TestSyncURLReadError(t *testing.T) {
	git := readyGit()
	git.urlErr = errString("url failed")
	auth := gitexec.HTTPSAuth{Token: "sekret"}
	lines, bad, err := runOne(t, git, 0, resolved.OnErrorContinue, auth, nil, nil)
	if err != nil || !bad || git.lses != 0 || !strings.Contains(lines[0], "url failed") {
		t.Fatalf("%v %v %q", bad, err, lines)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
