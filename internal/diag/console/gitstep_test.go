package console

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatGitStep(t *testing.T) {
	cases := []struct {
		name    string
		step    string
		done    int
		total   int
		label   string
		attempt int
		elapsed time.Duration
		want    string
	}{
		{
			name: "first", step: "pull", done: 3, total: 12,
			label: "documentation/architecture", attempt: 1,
			elapsed: 5 * time.Second, want: "pull 3/12 documentation/architecture 0:05",
		},
		{
			name: "second", step: "pull", done: 3, total: 12,
			label: "documentation/architecture", attempt: 2,
			elapsed: 41 * time.Second,
			want:    "pull 3/12 documentation/architecture attempt 2 0:41",
		},
		{
			name: "over hour", step: "push main", done: 0, total: 1,
			label: "my repo", attempt: 1,
			elapsed: 61*time.Minute + 5*time.Second,
			want:    "push main 0/1 my repo 61:05",
		},
		{
			name: "zero pad seconds", step: "clone", done: 0, total: 2,
			label: "group/api", attempt: 1, elapsed: 0,
			want: "clone 0/2 group/api 0:00",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatGitStep(tc.step, tc.done, tc.total, tc.label, tc.attempt, tc.elapsed)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestGitStepPlainLineStays(t *testing.T) {
	var buf bytes.Buffer
	cur := time.Unix(0, 0)
	g := NewGitStep(&buf, false, 12)
	g.now = func() time.Time { return cur }
	g.Start("pull", "documentation/architecture", 1)
	cur = cur.Add(time.Minute)
	g.repaint()
	g.Clear()
	g.Start("push", "documentation/architecture", 1)
	got := buf.String()
	want := "pull 0/12 documentation/architecture 0:00\npush 0/12 documentation/architecture 0:00\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestGitStepTerminalRedrawAndClear(t *testing.T) {
	var buf bytes.Buffer
	cur := time.Unix(0, 0)
	g := NewGitStep(&buf, true, 12)
	g.now = func() time.Time { return cur }
	g.ticks = make(chan struct{})
	painted := make(chan struct{}, 1)
	g.onPaint = func() { painted <- struct{}{} }

	g.Start("pull", "documentation/architecture", 2)
	if got := buf.String(); got != "\rpull 0/12 documentation/architecture attempt 2 0:00\033[K" {
		t.Fatalf("start %q", got)
	}
	cur = cur.Add(41 * time.Second)
	g.ticks <- struct{}{}
	<-painted
	if !strings.HasSuffix(buf.String(), "\rpull 0/12 documentation/architecture attempt 2 0:41\033[K") {
		t.Fatalf("tick %q", buf.String())
	}
	g.Clear()
	if !strings.HasSuffix(buf.String(), "\r\033[K") {
		t.Fatalf("clear %q", buf.String())
	}
	buf.Reset()
	g.Start("push", "documentation/architecture", 1)
	if buf.String() != "\rpush 0/12 documentation/architecture 0:00\033[K" {
		t.Fatalf("replace %q", buf.String())
	}
	g.Stop()
}

func TestGitStepMarkDoneAndNil(t *testing.T) {
	var buf bytes.Buffer
	g := NewGitStep(&buf, false, 2)
	g.Start("clone", "group/a", 1)
	var line string
	g.Result(func(s string) { line = s }, "✔ group/a")
	g.Start("clone", "group/b", 1)
	if line != "✔ group/a" {
		t.Fatal(line)
	}
	want := "clone 0/2 group/a 0:00\nclone 1/2 group/b 0:00\n"
	if buf.String() != want {
		t.Fatalf("got %q", buf.String())
	}
	var sink *GitStep
	called := false
	sink.Start("clone", "a", 1)
	sink.Stop()
	sink.Clear()
	sink.MarkDone()
	sink.SetTotal(3)
	sink.Result(func(string) { called = true }, "✔ a")
	if !called {
		t.Fatal("nil result dropped the line")
	}
}
