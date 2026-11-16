package publish

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/cli"
	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
)

type fakeGit struct {
	head        string
	detached    bool
	branches    map[string]bool
	remote      map[string]string
	urlsOf      func(dir string) []string
	pushes      []string
	adds        []string
	renames     []string
	pushFail    map[string]string
	failRemotes error
}

func (f *fakeGit) Remotes(_ context.Context, dir string) ([]string, string, error) {
	if f.failRemotes != nil {
		return nil, "", f.failRemotes
	}
	if f.urlsOf != nil {
		return f.urlsOf(dir), "", nil
	}
	return nil, "", nil
}

func (f *fakeGit) Head(context.Context, string) (string, bool, string, error) {
	return f.head, f.detached, "", nil
}

func (f *fakeGit) HasBranch(_ context.Context, _, name string) (bool, string, error) {
	return f.branches[name], "", nil
}

func (f *fakeGit) RemoteURL(_ context.Context, _, name string) (string, bool, string, error) {
	if f.remote == nil {
		return "", true, "", nil
	}
	u, ok := f.remote[name]
	if !ok {
		return "", true, "", nil
	}
	return u, false, "", nil
}

func (f *fakeGit) RemoteAdd(_ context.Context, _, name, remoteURL string) (string, error) {
	if f.remote == nil {
		f.remote = map[string]string{}
	}
	f.remote[name] = remoteURL
	f.adds = append(f.adds, remoteURL)
	return "", nil
}

func (f *fakeGit) RemoteRename(_ context.Context, _, oldName, newName string) (string, error) {
	if f.remote == nil {
		return "", fmt.Errorf("no remote")
	}
	u, ok := f.remote[oldName]
	if !ok {
		return "", fmt.Errorf("no remote")
	}
	delete(f.remote, oldName)
	f.remote[newName] = u
	f.renames = append(f.renames, newName)
	return "", nil
}

func (f *fakeGit) Push(_ context.Context, _, _, branch string) (string, error) {
	f.pushes = append(f.pushes, branch)
	if msg, ok := f.pushFail[branch]; ok {
		delete(f.pushFail, branch)
		return msg, fmt.Errorf("%s", msg)
	}
	return "", nil
}

type fakeRepo struct {
	have      map[string]bool
	calls     []string
	preflight []error
	preCalls  int
	writes    []string
}

func (f *fakeRepo) Exists(_ context.Context, path, name string) (bool, error) {
	if f.have == nil {
		f.have = map[string]bool{}
	}
	key := path + "/" + name
	f.calls = append(f.calls, "exists "+key)
	return f.have[key], nil
}

func (f *fakeRepo) Create(_ context.Context, path, name string) error {
	if f.have == nil {
		f.have = map[string]bool{}
	}
	key := path + "/" + name
	f.calls = append(f.calls, "create "+key)
	f.have[key] = true
	return nil
}

func (f *fakeRepo) Preflight(context.Context, string, string) error {
	f.preCalls++
	if len(f.preflight) == 0 {
		return nil
	}
	err := f.preflight[0]
	f.preflight = f.preflight[1:]
	return err
}

func (f *fakeRepo) WriteProperty(_ context.Context, _, _, _, value string) error {
	f.writes = append(f.writes, value)
	return nil
}

func TestPublishCopiesSkipRenameAndPush(t *testing.T) {
	prev := unixNow
	unixNow = func() int64 { return 100 }
	t.Cleanup(func() { unixNow = prev })

	git := &fakeGit{
		head:     "develop",
		branches: map[string]bool{"main": true},
		remote:   map[string]string{"origin": "https://old.example/acme/api.git"},
	}
	repo := &fakeRepo{}
	var lines []string
	job := &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", attr: "api", addr: "git@github.com:acme/api.git"}
	failed, err := publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "workspace-path", 0, resolved.OnErrorContinue, git, repo, repo, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || failed || len(git.pushes) != 0 || len(repo.writes) != 1 || repo.writes[0] != "api" {
		t.Fatalf("failed=%v err=%v pushes=%v writes=%v", failed, err, git.pushes, repo.writes)
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "➡️ ") {
		t.Fatalf("lines %v", lines)
	}
	if len(repo.calls) != 2 || !strings.HasPrefix(repo.calls[1], "create ") {
		t.Fatalf("calls %v", repo.calls)
	}

	lines = nil
	git.remote["origin"] = "https://old.example/acme/api.git"
	job = &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err = publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishRename, "", 0, resolved.OnErrorContinue, git, repo, nil, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || failed || len(git.renames) != 1 || git.renames[0] != "origin_100" {
		t.Fatalf("failed=%v err=%v renames=%v", failed, err, git.renames)
	}
	if len(git.pushes) != 2 || git.pushes[0] != "main" || git.pushes[1] != "develop" {
		t.Fatalf("pushes %v", git.pushes)
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "✔ ") {
		t.Fatalf("lines %v", lines)
	}
}

func TestPublishCopiesPartialAndRetry(t *testing.T) {
	git := &fakeGit{
		head:     "develop",
		branches: map[string]bool{"main": true},
		pushFail: map[string]string{"develop": "hook declined"},
	}
	repo := &fakeRepo{}
	var lines []string
	job := &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err := publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "", 0, resolved.OnErrorContinue, git, repo, nil, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || !failed || len(lines) != 1 || !strings.HasPrefix(lines[0], "⚠ ") || !strings.Contains(lines[0], "hook declined") {
		t.Fatalf("failed=%v err=%v lines=%v", failed, err, lines)
	}

	git.pushFail = map[string]string{"main": "no upstream", "develop": "later"}
	lines = nil
	job = &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err = publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "", 0, resolved.OnErrorContinue, git, repo, nil, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || !failed || len(lines) != 1 || !strings.HasPrefix(lines[0], "✖ ") || !strings.Contains(lines[0], "no upstream") {
		t.Fatalf("failed=%v err=%v lines=%v pushes=%v", failed, err, lines, git.pushes)
	}

	git.pushes = nil
	git.pushFail = map[string]string{"develop": "once"}
	lines = nil
	job = &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err = publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "", 1, resolved.OnErrorAbort, git, repo, nil, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || failed || len(lines) != 1 || !strings.HasPrefix(lines[0], "✔ ") {
		t.Fatalf("failed=%v err=%v lines=%v pushes=%v", failed, err, lines, git.pushes)
	}
	if len(git.pushes) != 4 {
		t.Fatalf("pushes %v", git.pushes)
	}
}

func TestPublishCopiesMasterAndCurrentOnly(t *testing.T) {
	git := &fakeGit{head: "topic", branches: map[string]bool{"master": true}}
	repo := &fakeRepo{have: map[string]bool{"acme/api": true}}
	var lines []string
	job := &publishJob{dir: "/c", rel: ".", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err := publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "", 0, resolved.OnErrorContinue, git, repo, nil, nil, func(s string) {
		lines = append(lines, s)
	})
	if err != nil || failed || strings.Join(git.pushes, ",") != "master,topic" {
		t.Fatalf("failed=%v err=%v pushes=%v", failed, err, git.pushes)
	}
	if joined := strings.Join(repo.calls, ","); strings.Contains(joined, "create") {
		t.Fatalf("created existing repo: %v", repo.calls)
	}

	git = &fakeGit{head: "topic", branches: map[string]bool{}}
	job = &publishJob{dir: "/c", rel: "api", path: "acme", name: "api", addr: "git@github.com:acme/api.git"}
	failed, err = publishCopies(context.Background(), []*publishJob{job}, "origin", cli.PublishSkip, "", 0, resolved.OnErrorAbort, git, repo, nil, nil, func(string) {})
	if err != nil || failed || strings.Join(git.pushes, ",") != "topic" {
		t.Fatalf("failed=%v err=%v pushes=%v", failed, err, git.pushes)
	}
}

var _ scm.RepositoryCreator = (*fakeRepo)(nil)
