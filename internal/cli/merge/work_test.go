package merge

import (
	"context"
	"testing"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/gitexec"
)

func sample() cli.MergeSettings {
	return cli.MergeSettings{From: "feature", To: "main", ToSet: true, Remote: "origin"}
}

func TestMergeExamples(t *testing.T) {
	ctx := context.Background()

	moved := &fakeGit{entryBranch: "other", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "bbb"}
	res := handleMerge(ctx, "dir", sample(), authNone(), moved, nil, nil, "documentation/architecture")
	got := formatMerge(res.signs, "documentation/architecture", res.notes)
	if got != "✔️ ✔️ ✔️ ✔️ ➡️ documentation/architecture" || moved.deletes != 1 || len(moved.switchReqs) != 1 {
		t.Fatalf("%q deletes %d switches %d", got, moved.deletes, len(moved.switchReqs))
	}
	if moved.switchReqs[0].NoGuess || moved.switchReqs[0].Target != "main" {
		t.Fatalf("%+v", moved.switchReqs[0])
	}

	same := &fakeGit{entryBranch: "main", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "aaa"}
	res = handleMerge(ctx, "dir", sample(), authNone(), same, nil, nil, "documentation/glossary")
	got = formatMerge(res.signs, "documentation/glossary", res.notes)
	if got != "✔️ ➡️ ➡️ ✔️ ➡️ documentation/glossary" || same.deletes != 1 {
		t.Fatalf("%q deletes %d", got, same.deletes)
	}

	conflict := &fakeGit{entryBranch: "other", afterBranch: "main", oidAfterSwitch: "aaa", unmerged: []string{"foo.md", "bar.go"}, mergeHead: true}
	stay := sample()
	stay.Stay = true
	res = handleMerge(ctx, "dir", stay, authNone(), conflict, nil, nil, "documentation/specifications")
	got = formatMerge(res.signs, "documentation/specifications", res.notes)
	want := "✔️ ✔️ ⚔️ ⏩ ⏩ documentation/specifications\n    ⚔️ foo.md, bar.go"
	if got != want || conflict.deletes != 0 || len(conflict.switchReqs) != 1 {
		t.Fatalf("%q deletes %d switches %d", got, conflict.deletes, len(conflict.switchReqs))
	}

	dirty := &fakeGit{entryBranch: "main", dirty: true}
	res = handleMerge(ctx, "dir", sample(), authNone(), dirty, nil, nil, "documentation/product-intent")
	got = formatMerge(res.signs, "documentation/product-intent", res.notes)
	want = "⏩ ⏩ ⏩ ⏩ ⏩ documentation/product-intent\n    uncommitted changes"
	if got != want || !res.precheck || dirty.fetches != 0 {
		t.Fatalf("%q precheck %v fetches %d", got, res.precheck, dirty.fetches)
	}
}

func TestMergeStayKeepAndRestore(t *testing.T) {
	ctx := context.Background()
	s := sample()
	s.Stay = true
	s.KeepSource = true
	git := &fakeGit{entryBranch: "topic", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "bbb"}
	res := handleMerge(ctx, "dir", s, authNone(), git, nil, nil, "copy")
	if res.signs != [5]string{signOK, signOK, signOK, signSame, signOK} || git.deletes != 0 || len(git.switchReqs) != 2 {
		t.Fatalf("%v deletes %d switches %+v", res.signs, git.deletes, git.switchReqs)
	}
	if git.switchReqs[1].Target != "topic" || git.switchReqs[1].Detach {
		t.Fatalf("%+v", git.switchReqs[1])
	}

	s.KeepSource = false
	s.From = "topic"
	s.To = "main"
	git = &fakeGit{entryBranch: "topic", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "bbb"}
	res = handleMerge(ctx, "dir", s, authNone(), git, nil, nil, "copy")
	if git.deletes != 0 || res.signs[3] != signSame || res.signs[4] != signOK {
		t.Fatalf("signs %v deletes %d", res.signs, git.deletes)
	}

	failed := &fakeGit{entryBranch: "other", afterBranch: "main", oidAfterSwitch: "aaa", mergeExit: 1, mergeStderr: "merge failed"}
	res = handleMerge(ctx, "dir", sample(), authNone(), failed, nil, nil, "copy")
	if res.signs[2] != signFail || res.signs[3] != signSkip || res.signs[4] != signOK || len(failed.switchReqs) != 2 || failed.deletes != 0 {
		t.Fatalf("%v switches %d", res.signs, len(failed.switchReqs))
	}
	if failed.switchReqs[1].Target != "other" {
		t.Fatalf("%+v", failed.switchReqs[1])
	}

	detached := &fakeGit{entryDetach: true, entryOID: "abc", afterBranch: "main", oidAfterSwitch: "aaa", oidAfterMerge: "bbb"}
	s = sample()
	s.Stay = true
	res = handleMerge(ctx, "dir", s, authNone(), detached, nil, nil, "copy")
	if detached.deletes != 1 || len(detached.switchReqs) != 2 || !detached.switchReqs[1].Detach || detached.switchReqs[1].Target != "abc" {
		t.Fatalf("deletes %d reqs %+v", detached.deletes, detached.switchReqs)
	}
}

func TestMergeFetchAndSwitchFailures(t *testing.T) {
	ctx := context.Background()
	fetch := &fakeGit{entryBranch: "main", fetchExit: 128, fetchStderr: "network"}
	res := handleMerge(ctx, "dir", sample(), authNone(), fetch, nil, nil, "copy")
	if res.signs[0] != signFail || fetch.fetches != 1 || len(fetch.switchReqs) != 0 {
		t.Fatalf("%v fetches %d", res.signs, fetch.fetches)
	}
	if res.notes[0] != "network" {
		t.Fatalf("%v", res.notes)
	}

	sw := &fakeGit{entryBranch: "other", switchExit: 1, switchStderr: "switch failed"}
	res = handleMerge(ctx, "dir", sample(), authNone(), sw, nil, nil, "copy")
	if res.signs[1] != signFail || len(sw.switchReqs) != 1 || sw.merges != 0 {
		t.Fatalf("%v switches %d merges %d", res.signs, len(sw.switchReqs), sw.merges)
	}

	read := &fakeGit{entryBranch: "other", afterSymErr: errString("git failed"), oidAfterSwitch: "aaa"}
	res = handleMerge(ctx, "dir", sample(), authNone(), read, nil, nil, "copy")
	if res.signs[1] != signFail || res.notes[0] != "working copy check failed" || len(read.switchReqs) != 2 || read.merges != 0 {
		t.Fatalf("%v notes %v switches %d", res.signs, res.notes, len(read.switchReqs))
	}
}

func TestMergeOptsAndToken(t *testing.T) {
	s := sample()
	s.GitOpts = []string{"--quiet"}
	s.FetchOpts = []string{"--prune"}
	s.MergeOpts = []string{"--ff-only"}
	s.BranchOpts = []string{"-q"}
	git := &fakeGit{
		entryBranch:    "other",
		afterBranch:    "main",
		oidAfterSwitch: "aaa",
		oidAfterMerge:  "bbb",
		remoteURL:      "https://github.com/acme/api.git",
	}
	auth := authNone()
	auth.Token = "sekret"
	auth.Upstream = "github"
	res := handleMerge(context.Background(), "dir", s, auth, git, nil, nil, "copy")
	if res.failed() {
		t.Fatalf("%v %v", res.signs, res.notes)
	}
	if len(git.lastCfg) != 2 || git.lastFetch[0] != "--quiet" || git.lastFetch[1] != "--prune" || git.lastRemote != "origin" {
		t.Fatalf("cfg %v opts %v remote %s", git.lastCfg, git.lastFetch, git.lastRemote)
	}
	if git.lastMerge[0] != "--quiet" || git.lastMerge[1] != "--ff-only" {
		t.Fatalf("merge opts %v", git.lastMerge)
	}
	if git.lastDelete[0] != "--quiet" || git.lastDelete[1] != "-q" {
		t.Fatalf("delete opts %v", git.lastDelete)
	}
}

func authNone() gitexec.HTTPSAuth {
	return gitexec.HTTPSAuth{}
}
