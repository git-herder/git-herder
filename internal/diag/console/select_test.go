package console

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"golang.org/x/term"
)

func TestPickerFilterAndToggle(t *testing.T) {
	p := &picker{
		items:    []SelectItem{{Text: "g/Api", URL: "https://example/a"}, {Text: "g/web", URL: "https://example/w"}},
		on:       make([]bool, 2),
		terminal: true,
	}
	if len(p.visible()) != 2 {
		t.Fatal(p.visible())
	}
	p.onKey(keys.RuneKey, "API")
	vis := p.visible()
	if len(vis) != 1 || vis[0] != 0 {
		t.Fatalf("%v", vis)
	}
	p.onKey(keys.Space, "")
	if !p.on[0] {
		t.Fatal("not checked")
	}
	p.onKey(keys.Backspace, "")
	p.onKey(keys.Backspace, "")
	p.onKey(keys.Backspace, "")
	if p.filter != "" || len(p.visible()) != 2 {
		t.Fatalf("filter %q vis %v", p.filter, p.visible())
	}
	if got := p.selected(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("%v", got)
	}
	view := p.render()
	if !strings.Contains(view, "[x]") || !strings.Contains(view, "https://example/a") {
		t.Fatalf("%s", view)
	}
	p.onKey(keys.Down, "")
	p.onKey(keys.Up, "")
	if p.cursor != 0 {
		t.Fatalf("cursor %d", p.cursor)
	}
	stop, cancel := p.onKey(keys.Escape, "")
	if !stop || !cancel {
		t.Fatal("esc")
	}
	stop, cancel = p.onKey(keys.Enter, "")
	if !stop || cancel {
		t.Fatal("enter")
	}
}

func TestRunSelectKeys(t *testing.T) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("real keyboard")
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = keyboard.SimulateKeyPress(keys.Key{Code: keys.Space})
		_ = keyboard.SimulateKeyPress(keys.Key{Code: keys.Down})
		_ = keyboard.SimulateKeyPress(keys.Key{Code: keys.Enter})
	}()
	idx, err := runSelect([]SelectItem{{Text: "a", URL: "https://e/a"}, {Text: "b", URL: "https://e/b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 1 || idx[0] != 0 {
		t.Fatalf("%v", idx)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = keyboard.SimulateKeyPress(keys.Key{Code: keys.Escape})
	}()
	if _, err := runSelect([]SelectItem{{Text: "a", URL: "https://e/a"}}); !errors.Is(err, ErrSelectCancelled) {
		t.Fatal(err)
	}
}

func TestSelectWithoutTerminal(t *testing.T) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		t.Skip("stdin and stdout are a terminal")
	}
	if _, err := New().Select([]SelectItem{{Text: "a"}}); err == nil {
		t.Fatal("expected unavailable input")
	}
}
