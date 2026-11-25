package workspace

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/diag/console"
)

func TestNameAndPath(t *testing.T) {
	dir := filepath.Clean(filepath.FromSlash("/qwe/rty/uio/dfg/asdsd"))
	c := Copy{Dir: dir}
	if c.Name() != "asdsd" {
		t.Fatalf("name %q", c.Name())
	}
	want := filepath.Clean(filepath.FromSlash("/qwe/rty/uio/dfg"))
	if c.Path() != want {
		t.Fatalf("path %q", c.Path())
	}
}

func TestWalkOrderAndStop(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a2"))
	makeRepo(t, filepath.Join(root, "a10"))
	makeRepo(t, filepath.Join(root, "b", "child"))
	makeRepo(t, filepath.Join(root, "c"))
	makeRepo(t, filepath.Join(root, "c", "nested"))
	makeRepo(t, filepath.Join(root, "d", "sub"))
	if err := os.WriteFile(filepath.Join(root, "d", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(root, "notrepo")
	if err := os.MkdirAll(gitFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitFile, ".git"), []byte("gitdir: x"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeRepo(t, filepath.Join(gitFile, "real"))

	got, err := Resolve().Workspace(root).Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "a10"),
		filepath.Join(root, "a2"),
		filepath.Join(root, "b", "child"),
		filepath.Join(root, "c"),
		filepath.Join(root, "d", "sub"),
		filepath.Join(root, "notrepo", "real"),
	}
	assertDirs(t, got, want)
}

func TestRootGitIsOnlyCopy(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root)
	makeRepo(t, filepath.Join(root, "other"))
	got, err := Resolve().Workspace(root + string(os.PathSeparator)).Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{filepath.Clean(root)})
}

func TestSymlinkDirIsNotDescended(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	makeRepo(t, target)
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	makeRepo(t, filepath.Join(root, "real"))
	got, err := Resolve().Workspace(root).Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{filepath.Join(root, "real")})
}

func TestFiltersAndRegexFallback(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "selalpha", "selteam", "api"))
	makeRepo(t, filepath.Join(root, "selalpha", "web"))
	makeRepo(t, filepath.Join(root, "selteam", "api"))
	makeRepo(t, filepath.Join(root, "selalpha", "selteam", "api2"))
	makeRepo(t, filepath.Join(root, "selalpha", "selteam", "foo(bar"))

	both, err := Resolve().
		Workspace(root).
		Path("selalpha", "selteam").
		Name("^api$", "i").
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, both, []string{filepath.Join(root, "selalpha", "selteam", "api")})

	sub, err := Resolve().Workspace(root).Name("(").Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, sub, []string{filepath.Join(root, "selalpha", "selteam", "foo(bar")})
}

func TestCriteriaDropsAndStops(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a"))
	makeRepo(t, filepath.Join(root, "b"))
	picker := &fakePicker{}

	got, err := Resolve().
		Workspace(root).
		Name("b").
		Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{filepath.Join(root, "b")})

	calls := 0
	got, err = Resolve().
		Workspace(root).
		Git(boomGit{onVerify: func() { calls++ }}).
		Interactive(true).
		Picker(picker).
		HasBranch("develop").
		Find(context.Background())
	if err == nil || err.Error() != "boom" || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
	if picker.called {
		t.Fatal("picker ran after criterion error")
	}
}

func TestInteractive(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a"))
	makeRepo(t, filepath.Join(root, "b"))
	makeRepo(t, filepath.Join(root, "c"))

	idle := &fakePicker{}
	got, err := Resolve().Workspace(root).Picker(idle).Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if idle.called || len(got) != 3 {
		t.Fatalf("called %v len %d", idle.called, len(got))
	}

	idle = &fakePicker{}
	got, err = Resolve().
		Workspace(root).
		Name("^missing$").
		Interactive(true).
		Picker(idle).
		Find(context.Background())
	if err != nil || len(got) != 0 || idle.called {
		t.Fatalf("got %v err %v called %v", got, err, idle.called)
	}

	pick := &fakePicker{idx: []int{0, 2}}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(pick).Find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertDirs(t, got, []string{filepath.Join(root, "a"), filepath.Join(root, "c")})
	if len(pick.items) != 3 || pick.items[1].Text != filepath.Join(root, "b") || pick.items[1].URL != "" {
		t.Fatalf("items %+v", pick.items)
	}

	none := &fakePicker{idx: []int{}}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(none).Find(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty enter got %v err %v", got, err)
	}

	cancel := &fakePicker{err: console.ErrSelectCancelled}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(cancel).Find(context.Background())
	if !errors.Is(err, ErrCancelled) || !errors.Is(err, console.ErrSelectCancelled) || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}

	unavailable := &fakePicker{err: errors.New("interactive input is not available")}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(unavailable).Find(context.Background())
	if err == nil || errors.Is(err, ErrCancelled) || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}

	_, err = Resolve().Workspace(root).Interactive(true).Find(context.Background())
	if err == nil || err.Error() != "interactive selection source is not set" {
		t.Fatal(err)
	}

	bad := &fakePicker{idx: []int{-1}}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(bad).Find(context.Background())
	if err == nil || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}
	bad.idx = []int{9}
	got, err = Resolve().Workspace(root).Interactive(true).Picker(bad).Find(context.Background())
	if err == nil || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestReadDirErrorDropsPartial(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, filepath.Join(root, "a"))
	if err := os.Mkdir(filepath.Join(root, "denied"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := readDir
	t.Cleanup(func() { readDir = orig })
	readDir = func(name string) ([]os.DirEntry, error) {
		if filepath.Base(name) == "denied" {
			return nil, os.ErrPermission
		}
		return os.ReadDir(name)
	}
	got, err := Resolve().Workspace(root).Find(context.Background())
	if err == nil || !errors.Is(err, os.ErrPermission) || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	got, err = Resolve().Workspace(missing).Find(context.Background())
	if err == nil || got != nil {
		t.Fatalf("missing got %v err %v", got, err)
	}
}

func TestEmptyWorkspace(t *testing.T) {
	text := captureStderr(t, func() {
		got, err := Resolve().Workspace(t.TempDir()).Find(context.Background())
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
	if !strings.Contains(text, "no matching working copies") {
		t.Fatalf("stderr %q", text)
	}

	root := t.TempDir()
	makeRepo(t, root)
	text = captureStderr(t, func() {
		got, err := Resolve().Workspace(root).Find(context.Background())
		if err != nil || len(got) != 1 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
	if strings.Contains(text, "no matching working copies") {
		t.Fatalf("warned for a found copy: %q", text)
	}

	text = captureStderr(t, func() {
		missing := filepath.Join(t.TempDir(), "missing")
		if _, err := Resolve().Workspace(missing).Find(context.Background()); err == nil {
			t.Fatal("expected read error")
		}
	})
	if strings.Contains(text, "no matching working copies") {
		t.Fatalf("warned on error: %q", text)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

type fakePicker struct {
	idx    []int
	err    error
	called bool
	items  []console.SelectItem
}

func (f *fakePicker) Select(items []console.SelectItem) ([]int, error) {
	f.called = true
	f.items = items
	return f.idx, f.err
}

func makeRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertDirs(t *testing.T, got []Copy, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len %d want %d (%v)", len(got), len(want), dirsOf(got))
	}
	for i := range want {
		if got[i].Dir != want[i] {
			t.Fatalf("[%d] %q want %q", i, got[i].Dir, want[i])
		}
	}
}

func dirsOf(copies []Copy) []string {
	out := make([]string, len(copies))
	for i, c := range copies {
		out[i] = c.Dir
	}
	return out
}
