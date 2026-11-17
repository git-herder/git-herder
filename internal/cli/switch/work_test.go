package switchcmd

import (
	"context"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
)

func TestSwitchExamples(t *testing.T) {
	ctx := context.Background()
	branch := "feature"

	changed := &fakeGit{symName: "main", symAfter: branch, oid: "aaa", oidAfter: "bbb"}
	sw, up, notes := handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: branch}, changed, nil, nil, "documentation/architecture")
	if got := formatSwitch(sw, up, "documentation/architecture", notes); got != "✔️ ➡️ documentation/architecture" {
		t.Fatalf("%q switches %d", got, changed.switches)
	}
	if !changed.lastReq.NoGuess || changed.lastReq.Target != branch {
		t.Fatalf("%+v", changed.lastReq)
	}

	same := &fakeGit{symName: branch, symAfter: branch, oid: "aaa", upExit: 1}
	sw, up, notes = handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: branch, TrackSet: true, Track: "origin"}, same, nil, nil, "documentation/glossary")
	if got := formatSwitch(sw, up, "documentation/glossary", notes); got != "➡️ ✔️ documentation/glossary" {
		t.Fatalf("%q sets %d", got, same.sets)
	}
	if same.sets != 1 || same.upCalls != 1 {
		t.Fatalf("up %d sets %d", same.upCalls, same.sets)
	}

	conflict := &fakeGit{symName: "main", oid: "aaa", unmerged: []string{"foo.md", "bar.go"}}
	sw, up, notes = handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: branch, Merge: true}, conflict, nil, nil, "documentation/specifications")
	got := formatSwitch(sw, up, "documentation/specifications", notes)
	want := "⚔️ ⏩ documentation/specifications\n    ⚔️ foo.md, bar.go"
	if got != want {
		t.Fatalf("%q", got)
	}
	if conflict.upCalls != 0 {
		t.Fatal("upstream ran after conflict")
	}

	failed := &fakeGit{symName: "main", oid: "aaa", switchExit: 1, switchStderr: "switch failed"}
	sw, up, notes = handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: branch, TrackSet: true, Track: "origin"}, failed, nil, nil, "documentation/product-intent")
	got = formatSwitch(sw, up, "documentation/product-intent", notes)
	want = "❌ ⏩ documentation/product-intent\n    switch failed\n    did not start"
	if got != want {
		t.Fatalf("%q", got)
	}
	if failed.upCalls != 0 || failed.sets != 0 {
		t.Fatal("upstream ran after failed switch")
	}

	tracked := &fakeGit{symName: "main", symAfter: branch, oid: "aaa", oidAfter: "bbb", setExit: 1, setStderr: "upstream branch was not set"}
	sw, up, notes = handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: branch, TrackSet: true, Track: "origin"}, tracked, nil, nil, "documentation/adr")
	got = formatSwitch(sw, up, "documentation/adr", notes)
	want = "✔️ ❌ documentation/adr\n    upstream branch was not set"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestSwitchAlreadyTrackedAndDiscard(t *testing.T) {
	ctx := context.Background()
	git := &fakeGit{symName: "feature", symAfter: "feature", oid: "aaa", upLine: "origin/feature", dirty: true}
	sw, up, notes := handleSwitch(ctx, "dir", cli.SwitchSettings{Branch: "feature", TrackSet: true, Track: "origin", DiscardChanges: true}, git, nil, nil, "documentation/architecture")
	if got := formatSwitch(sw, up, "documentation/architecture", notes); got != "✔️ ➡️ documentation/architecture" {
		t.Fatalf("%q", got)
	}
	if git.sets != 0 {
		t.Fatal("set-upstream ran for an existing upstream")
	}
	if !git.lastReq.Discard {
		t.Fatal("discard flag missing")
	}
}

func TestSwitchCreatePassesStart(t *testing.T) {
	git := &fakeGit{symName: "main", symAfter: "feature", oid: "aaa", oidAfter: "bbb"}
	s := cli.SwitchSettings{Branch: "feature", StartPoint: "HEAD", StartPointSet: true, Create: true, GitOpts: []string{"--quiet"}}
	_, _, _ = handleSwitch(context.Background(), "dir", s, git, nil, nil, ".")
	if !git.lastReq.Create || git.lastReq.StartPoint != "HEAD" || len(git.lastReq.Opts) != 1 || git.lastReq.Opts[0] != "--quiet" {
		t.Fatalf("%+v", git.lastReq)
	}
}

func TestSwitchCheckFailureSkipsSwitch(t *testing.T) {
	git := &fakeGit{symErr: errString("git failed")}
	sw, up, notes := handleSwitch(context.Background(), "dir", cli.SwitchSettings{Branch: "feature"}, git, nil, nil, "x")
	got := formatSwitch(sw, up, "x", notes)
	if got != "❌ ⏩ x\n    working copy check failed\n    did not start" || git.switches != 0 {
		t.Fatalf("%q switches %d", got, git.switches)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
