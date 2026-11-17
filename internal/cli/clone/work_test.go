package clone

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/github"
)

func TestCloneSkipFullDir(t *testing.T) {
	ws := t.TempDir()
	dest := filepath.Join(ws, "g", "api")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	var calls int
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "g", URL: "https://example/g/api",
	}}, false, "ssh", existSkip, nil, 0, resolved.OnErrorContinue, func(context.Context, []string, string, string) (string, error) {
		calls++
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "", nil)
	if err != nil || failed {
		t.Fatalf("failed=%v err=%v", failed, err)
	}
	if calls != 0 || len(lines) != 1 || !strings.HasPrefix(lines[0], "➡️ ") {
		t.Fatalf("calls=%d lines=%v", calls, lines)
	}
}

func TestCloneDropAndRetryCleansPartial(t *testing.T) {
	ws := t.TempDir()
	dest := filepath.Join(ws, "g", "api")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls int
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "g", URL: "https://example/g/api",
	}}, false, "ssh", existDrop, []string{"--depth=1"}, 1, resolved.OnErrorContinue, func(_ context.Context, opts []string, repoURL, got string) (string, error) {
		calls++
		if len(opts) != 1 || opts[0] != "--depth=1" || repoURL != "git@example:g/api.git" || got != dest {
			t.Fatalf("opts=%v url=%s dest=%s", opts, repoURL, got)
		}
		if err := os.MkdirAll(got, 0o755); err != nil {
			return "", err
		}
		if calls == 1 {
			if err := os.WriteFile(filepath.Join(got, "partial"), []byte("p"), 0o644); err != nil {
				return "", err
			}
			return "fatal: boom\n", os.ErrInvalid
		}
		return "", nil
	}, func(string) {}, nil, func(line string) { lines = append(lines, line) }, "", nil)
	if err != nil || failed {
		t.Fatalf("failed=%v err=%v lines=%v", failed, err, lines)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	if _, err := os.Stat(filepath.Join(dest, "partial")); !os.IsNotExist(err) {
		t.Fatal("partial dir survived retry")
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "✔ ") {
		t.Fatalf("lines=%v", lines)
	}
}

func TestCloneFileIsNotQueued(t *testing.T) {
	ws := t.TempDir()
	file := filepath.Join(ws, "g", "api")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "g", URL: "https://example/g/api",
	}}, false, "ssh", existSkip, nil, 2, resolved.OnErrorAbort, func(context.Context, []string, string, string) (string, error) {
		t.Fatal("git should not run")
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "", nil)
	if err == nil || !failed || len(lines) != 1 || !strings.Contains(lines[0], "file") {
		t.Fatalf("failed=%v err=%v lines=%v", failed, err, lines)
	}
}

func TestShortReasonFallback(t *testing.T) {
	if shortReason("\n  \n") != failFallback {
		t.Fatal(shortReason("\n  \n"))
	}
	if shortReason("  fatal: no\n") != "fatal: no" {
		t.Fatal(shortReason("  fatal: no\n"))
	}
}

func TestCloneGitFailureLine(t *testing.T) {
	ws := t.TempDir()
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "g", URL: "https://example/g/api",
	}}, false, "ssh", existSkip, nil, 0, resolved.OnErrorContinue, func(context.Context, []string, string, string) (string, error) {
		return "\nfatal: denied\n", os.ErrInvalid
	}, func(string) {}, nil, func(line string) { lines = append(lines, line) }, "", nil)
	if err != nil || !failed || len(lines) != 1 || !strings.Contains(lines[0], "fatal: denied") {
		t.Fatalf("failed=%v err=%v lines=%v", failed, err, lines)
	}
}

type stubProps struct {
	v     github.PropertyValue
	err   error
	calls int
}

func (s *stubProps) ReadProperty(context.Context, string, string, string) (github.PropertyValue, error) {
	s.calls++
	return s.v, s.err
}

func TestCloneAttributePath(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyString, Value: `domain\billing\api`}}
	var got string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "other", Path: "acme", URL: "https://example/acme/other",
	}}, false, "ssh", existSkip, nil, 0, resolved.OnErrorContinue, func(_ context.Context, _ []string, _ string, dest string) (string, error) {
		got = dest
		return "", nil
	}, nil, nil, func(string) {}, "workspace-path", props)
	if err != nil || failed || props.calls != 1 {
		t.Fatalf("failed=%v err=%v calls=%d", failed, err, props.calls)
	}
	want := filepath.Join(ws, "domain", "billing", "api")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCloneAttributeAbsentUsesWorkspace(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyAbsent}}
	var got string
	_, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "acme", URL: "https://example/acme/api",
	}}, false, "ssh", existSkip, nil, 0, resolved.OnErrorContinue, func(_ context.Context, _ []string, _ string, dest string) (string, error) {
		got = dest
		return "", nil
	}, nil, nil, func(string) {}, "workspace-path", props)
	if err != nil {
		t.Fatal(err)
	}
	if got != ws {
		t.Fatalf("got %s want %s", got, ws)
	}
}

func TestCloneAttributeNotString(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyNotString}}
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "acme", URL: "https://example/acme/api",
	}}, false, "ssh", existSkip, nil, 2, resolved.OnErrorAbort, func(context.Context, []string, string, string) (string, error) {
		t.Fatal("git should not run")
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "workspace-path", props)
	if err == nil || !failed || props.calls != 1 || len(lines) != 1 || !strings.Contains(lines[0], "not a string") {
		t.Fatalf("failed=%v err=%v reads=%d lines=%v", failed, err, props.calls, lines)
	}
}

func TestCloneAttributeReadError(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{err: os.ErrInvalid}
	var lines []string
	var calls int
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "acme", URL: "https://example/acme/api",
	}}, false, "ssh", existSkip, nil, 1, resolved.OnErrorContinue, func(context.Context, []string, string, string) (string, error) {
		calls++
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "workspace-path", props)
	if err != nil || !failed || calls != 0 || props.calls != 2 || len(lines) != 1 || !strings.HasPrefix(lines[0], "✖ ") {
		t.Fatalf("failed=%v err=%v calls=%d reads=%d lines=%v", failed, err, calls, props.calls, lines)
	}
}

func TestCloneAttributeDotDotNotQueued(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyString, Value: `domain\..\x`}}
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "acme", URL: "https://example/acme/api",
	}}, false, "ssh", existSkip, nil, 2, resolved.OnErrorAbort, func(context.Context, []string, string, string) (string, error) {
		t.Fatal("git should not run")
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "workspace-path", props)
	if err == nil || !failed || props.calls != 1 || len(lines) != 1 {
		t.Fatalf("failed=%v err=%v reads=%d lines=%v", failed, err, props.calls, lines)
	}
}

func TestCloneAttributeBadOwnerNotRead(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyString, Value: "domain"}}
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{{
		Name: "api", Path: "acme/team", URL: "https://example/acme/team/api",
	}}, false, "ssh", existSkip, nil, 2, resolved.OnErrorContinue, func(context.Context, []string, string, string) (string, error) {
		t.Fatal("git should not run")
		return "", nil
	}, nil, nil, func(line string) { lines = append(lines, line) }, "workspace-path", props)
	if err != nil || !failed || props.calls != 0 || len(lines) != 1 {
		t.Fatalf("failed=%v err=%v reads=%d lines=%v", failed, err, props.calls, lines)
	}
}

func TestEnsureWorkspace(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "new")
	if err := ensureWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorkspace(file); err == nil {
		t.Fatal("file workspace")
	}
}

func TestPickWithoutTerminal(t *testing.T) {
	_, err := pick(console.New(), []card.Card{{Name: "api", Path: "g", URL: "https://example/g/api"}})
	if err == nil {
		t.Fatal("expected interactive error")
	}
}

func TestCloneStepStatus(t *testing.T) {
	ws := t.TempDir()
	props := &stubProps{v: github.PropertyValue{State: github.PropertyString, Value: "slot"}}
	var buf bytes.Buffer
	steps := console.NewGitStep(&buf, false, 0)
	n := 0
	var lines []string
	failed, err := cloneCards(context.Background(), ws, []card.Card{
		{Name: "a", Path: "acme", URL: "https://token@example/acme/a"},
		{Name: "b", Path: "acme", URL: "https://example/acme/b"},
	}, false, "ssh", existSkip, nil, 1, resolved.OnErrorContinue, func(context.Context, []string, string, string) (string, error) {
		n++
		if n == 2 {
			return "fatal: boom", os.ErrInvalid
		}
		return "", nil
	}, nil, steps, func(line string) { lines = append(lines, line) }, "workspace-path", props)
	if err != nil || failed {
		t.Fatalf("failed=%v err=%v lines=%v", failed, err, lines)
	}
	if props.calls != 2 {
		t.Fatalf("attribute reads %d", props.calls)
	}
	want := strings.Join([]string{
		"clone 0/2 acme/a 0:00",
		"clone 1/2 acme/b 0:00",
		"clone 1/2 acme/b attempt 2 0:00",
		"",
	}, "\n")
	if buf.String() != want {
		t.Fatalf("status %q", buf.String())
	}
	if strings.Contains(buf.String(), "https://") || strings.Contains(buf.String(), "token") {
		t.Fatalf("secret in status %q", buf.String())
	}
}
