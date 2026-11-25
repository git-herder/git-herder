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

	// Remotes
	urls      []string
	remoteOut string
	remoteErr error
	remotes   int
}

func (f *fakeGit) VerifyQuiet(context.Context, string, string) (bool, gitexec.Result, error) {
	f.verified = append(f.verified, "x")
	return f.verifyOK, f.verifyRes, f.verifyErr
}

func (f *fakeGit) SymbolicHead(context.Context, string) (string, gitexec.Result, error) {
	f.symCalls++
	return f.symName, f.symRes, f.symErr
}

func (f *fakeGit) Remotes(context.Context, string) ([]string, string, error) {
	f.remotes++
	return f.urls, f.remoteOut, f.remoteErr
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
			restore := InstallGitForTest(tc.git)
			t.Cleanup(restore)
			got, err := Resolve().Workspace(root).HasBranch("develop").Find(context.Background())
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := InstallGitForTest(tc.git)
			t.Cleanup(restore)
			got, err := Resolve().Workspace(root).OnBranch("develop").Find(context.Background())
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
		{"другой хост", &fakeGit{urls: []string{"git@elsewhere.example:acme/api.git"}}, true, false},
		{"свой хост", &fakeGit{urls: []string{"git@github.com:acme/api.git"}}, false, false},
		{"один из двух свой", &fakeGit{urls: []string{"git@elsewhere.example:acme/api.git", "https://github.com/acme/api.git"}}, false, false},
		{"сбой запуска", &fakeGit{remoteErr: errors.New("git remote")}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restore := InstallGitForTest(tc.git)
			t.Cleanup(restore)
			got, err := Resolve().
				Workspace(root).
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
		})
	}
}

// Первый отбрасывающий критерий исключает копию: остальные для неё не зовутся.
func TestCriteriaShortCircuit(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a"))
	git := &fakeGit{symName: "other", symRes: gitexec.Result{Exit: 0}}
	restore := InstallGitForTest(git)
	t.Cleanup(restore)

	got, err := Resolve().
		Workspace(root).
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
		urls:     []string{"git@github.com:acme/api.git"},
	}
	restore := InstallGitForTest(git)
	t.Cleanup(restore)

	got, err := Resolve().
		Workspace(root).
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

	git.urls = nil
	got, err = Resolve().
		Workspace(root).
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
	restore := InstallGitForTest(git)
	t.Cleanup(restore)

	got, err := Resolve().
		Workspace(root).
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

func (boomGit) Remotes(context.Context, string) ([]string, string, error) {
	return nil, "", nil
}
