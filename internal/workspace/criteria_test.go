package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/git-herder/git-herder/internal/gitexec"
)

// fakeGit — подмена GitExecutor: ответы на вызовы критериев задаются полями.
type fakeGit struct {
	// VerifyQuiet
	verifyOK  bool
	verifyRes gitexec.Result
	verifyErr error
	verified  []string

	// SymbolicHead
	symName  string
	symRes   gitexec.Result
	symErr   error
	symCalls int

	// RemoteNames / RemoteURL / RemotePushURL
	names     []string
	namesRes  gitexec.Result
	namesErr  error
	fetchURL  map[string]string
	pushURL   map[string]string
	urlOut    string
	urlErr    error
	urlErrOn  string
	pushErr   bool
	seenNames []string
	urlCalls  []string
}

func (f *fakeGit) VerifyQuiet(context.Context, string, string) (bool, gitexec.Result, error) {
	f.verified = append(f.verified, "x")
	return f.verifyOK, f.verifyRes, f.verifyErr
}

func (f *fakeGit) SymbolicHead(context.Context, string) (string, gitexec.Result, error) {
	f.symCalls++
	return f.symName, f.symRes, f.symErr
}

func (f *fakeGit) RemoteNames(context.Context, string) ([]string, gitexec.Result, error) {
	return f.names, f.namesRes, f.namesErr
}

func (f *fakeGit) RemoteURL(_ context.Context, _, name string) (string, bool, string, error) {
	f.seenNames = append(f.seenNames, name)
	f.urlCalls = append(f.urlCalls, "fetch "+name)
	return f.lookupURL(name, f.fetchURL)
}

func (f *fakeGit) RemotePushURL(_ context.Context, _, name string) (string, bool, string, error) {
	f.urlCalls = append(f.urlCalls, "push "+name)
	if f.pushErr && f.urlErr != nil && (f.urlErrOn == "" || f.urlErrOn == name) {
		return "", false, f.urlOut, f.urlErr
	}
	return f.lookupURL(name, f.pushURL)
}

func (f *fakeGit) lookupURL(name string, urls map[string]string) (string, bool, string, error) {
	if f.urlErr != nil && !f.pushErr && (f.urlErrOn == "" || f.urlErrOn == name) {
		return "", false, f.urlOut, f.urlErr
	}
	raw, ok := urls[name]
	if !ok {
		return "", true, f.urlOut, nil
	}
	return raw, false, f.urlOut, nil
}

func TestHasBranch(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)

	cases := []struct {
		name string
		git  *fakeGit
		keep bool
		fail bool
	}{
		{"есть ветка", &fakeGit{verifyOK: true}, true, false},
		{"нет ветки", &fakeGit{verifyOK: false, verifyRes: gitexec.Result{Exit: 1}}, false, false},
		{"иной код", &fakeGit{verifyErr: errors.New("git rev-parse"), verifyRes: gitexec.Result{Exit: 128}}, false, true},
		{"сбой запуска", &fakeGit{verifyErr: errors.New("git not found in PATH")}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve().Workspace(root).Git(tc.git).HasBranch("develop").Find(context.Background())
			if tc.fail {
				if err == nil || got != nil {
					t.Fatalf("got %v err %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !tc.keep && len(got) != 0 {
				t.Fatalf("kept %v", dirsOf(got))
			}
			if tc.keep && len(got) != 1 {
				t.Fatalf("dropped %v", dirsOf(got))
			}
		})
	}
}

func TestOnBranch(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)

	cases := []struct {
		name string
		git  *fakeGit
		keep bool
		fail bool
	}{
		{"та же ветка", &fakeGit{symName: "develop", symRes: gitexec.Result{Exit: 0}}, true, false},
		{"другая ветка", &fakeGit{symName: "main", symRes: gitexec.Result{Exit: 0}}, false, false},
		{"отделённый HEAD", &fakeGit{symRes: gitexec.Result{Exit: 1}}, false, false},
		{"сбой запуска", &fakeGit{symErr: errors.New("git not found in PATH")}, false, true},
		{"иной код", &fakeGit{symRes: gitexec.Result{Exit: 128, Stderr: "fatal: not a git repository"}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve().Workspace(root).Git(tc.git).OnBranch("develop").Find(context.Background())
			if tc.fail {
				if err == nil || got != nil {
					t.Fatalf("got %v err %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !tc.keep && len(got) != 0 {
				t.Fatalf("kept %v", dirsOf(got))
			}
			if tc.keep && len(got) != 1 {
				t.Fatalf("dropped %v", dirsOf(got))
			}
		})
	}
}

func TestNoRemoteOnHost(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)

	cases := []struct {
		name string
		git  *fakeGit
		keep bool
		fail bool
	}{
		{"ремотов нет", &fakeGit{}, true, false},
		{"другой хост", &fakeGit{names: []string{"origin"}, fetchURL: map[string]string{"origin": "git@elsewhere.example:acme/api.git"}}, true, false},
		{"свой хост", &fakeGit{names: []string{"origin"}, fetchURL: map[string]string{"origin": "git@github.com:acme/api.git"}}, false, false},
		{"свой хост только на push", &fakeGit{names: []string{"origin"}, fetchURL: map[string]string{"origin": "git@elsewhere.example:acme/api.git"}, pushURL: map[string]string{"origin": "git@github.com:acme/api.git"}}, false, false},
		{"один из двух свой", &fakeGit{names: []string{"origin", "upstream"}, fetchURL: map[string]string{"origin": "git@elsewhere.example:acme/api.git", "upstream": "https://github.com/acme/api.git"}}, false, false},
		{"пустой URL", &fakeGit{names: []string{"origin"}, fetchURL: map[string]string{"origin": "  "}}, true, false},
		{"сбой списка", &fakeGit{namesErr: errors.New("git remote")}, false, true},
		{"сбой get-url", &fakeGit{names: []string{"origin", "upstream"}, urlErr: errors.New("git remote get-url"), urlErrOn: "origin"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve().
				Workspace(root).
				Git(tc.git).
				NoRemoteOnHost(Host{Name: "github.com"}).
				Find(context.Background())
			if tc.fail {
				if err == nil || got != nil {
					t.Fatalf("got %v err %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !tc.keep && len(got) != 0 {
				t.Fatalf("kept %v", dirsOf(got))
			}
			if tc.keep && len(got) != 1 {
				t.Fatalf("dropped %v", dirsOf(got))
			}
			if tc.name == "сбой get-url" && len(tc.git.seenNames) != 1 {
				t.Fatalf("next remote was read after an error: %v", tc.git.seenNames)
			}
		})
	}
}

func TestNoRemoteOnHostPushErrorStops(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)
	git := &fakeGit{
		names:    []string{"origin", "upstream"},
		fetchURL: map[string]string{"origin": "git@elsewhere.example:acme/api.git"},
		urlErr:   errors.New("git remote get-url --push"),
		urlErrOn: "origin",
		pushErr:  true,
	}
	got, err := Resolve().
		Workspace(root).
		Git(git).
		NoRemoteOnHost(Host{Name: "github.com"}).
		Find(context.Background())
	if err == nil || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}
	if len(git.seenNames) != 1 || len(git.urlCalls) != 2 {
		t.Fatalf("calls %v", git.urlCalls)
	}
}

func TestCriteriaLog(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)
	git := &fakeGit{verifyOK: true, verifyRes: gitexec.Result{Stdout: "deadbeef\n", Stderr: "warn\n"}}
	var lines []string
	_, err := Resolve().
		Workspace(root).
		Git(git).
		Log(func(s string) { lines = append(lines, s) }).
		HasBranch("develop").
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "warn\n" || lines[1] != "deadbeef\n" {
		t.Fatalf("log %q", lines)
	}

	lines = nil
	quiet := &fakeGit{verifyOK: true}
	_, err = Resolve().Workspace(root).Git(quiet).Log(func(s string) { lines = append(lines, s) }).HasBranch("develop").Find(context.Background())
	if err != nil || len(lines) != 0 {
		t.Fatalf("empty log %q err %v", lines, err)
	}
	if _, err = Resolve().Workspace(root).Git(quiet).HasBranch("develop").Find(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestCriteriaShortCircuit(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a"))
	git := &fakeGit{symName: "other", symRes: gitexec.Result{Exit: 0}}
	got, err := Resolve().
		Workspace(root).
		Git(git).
		HasBranch("develop").
		OnBranch("develop").
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("kept %v", dirsOf(got))
	}
	if len(git.verified) != 1 {
		t.Fatalf("verified %d", len(git.verified))
	}
	if git.symCalls != 0 {
		t.Fatalf("second criterion ran after a drop: %d", git.symCalls)
	}
}

// Критерии связаны по И: копия остаётся, только если прошла каждый.
func TestCriteriaAllMustPass(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)
	git := &fakeGit{
		verifyOK: true,
		symName:  "develop",
		symRes:   gitexec.Result{Exit: 0},
		names:    []string{"origin"},
		fetchURL: map[string]string{"origin": "git@github.com:acme/api.git"},
	}

	got, err := Resolve().
		Workspace(root).
		Git(git).
		HasBranch("develop").
		OnBranch("develop").
		NoRemoteOnHost(Host{Name: "github.com"}).
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("remote criterion must drop the copy: %v", dirsOf(got))
	}

	git.names = nil
	git.fetchURL = nil
	got, err = Resolve().
		Workspace(root).
		Git(git).
		HasBranch("develop").
		OnBranch("develop").
		NoRemoteOnHost(Host{Name: "github.com"}).
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{root})
}

// Повтор HasBranch добавляет ещё одно условие: нужны обе ветки.
func TestHasBranchRepeat(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)
	git := &fakeGit{verifyOK: true}
	got, err := Resolve().
		Workspace(root).
		Git(git).
		HasBranch("develop").
		HasBranch("release/v2").
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{root})
	if len(git.verified) != 2 {
		t.Fatalf("both branches must be checked: %d", len(git.verified))
	}
}

// boomGit — GitExecutor, чей VerifyQuiet считает вызовы и падает.
type boomGit struct {
	onVerify func()
}

func (b boomGit) VerifyQuiet(context.Context, string, string) (bool, gitexec.Result, error) {
	if b.onVerify != nil {
		b.onVerify()
	}
	return false, gitexec.Result{}, errors.New("boom")
}

func (boomGit) SymbolicHead(context.Context, string) (string, gitexec.Result, error) {
	return "", gitexec.Result{}, nil
}

func (boomGit) RemoteNames(context.Context, string) ([]string, gitexec.Result, error) {
	return nil, gitexec.Result{}, nil
}

func (boomGit) RemoteURL(context.Context, string, string) (string, bool, string, error) {
	return "", false, "", nil
}

func (boomGit) RemotePushURL(context.Context, string, string) (string, bool, string, error) {
	return "", false, "", nil
}
