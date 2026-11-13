package console

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// SelectItem — строка интерактивного списка.
type SelectItem struct {
	Text string
	URL  string
}

// ErrSelectCancelled — пользователь нажал ESC.
var ErrSelectCancelled = errors.New("selection cancelled")

// Select показывает список с галочками.
// Изначально все сняты. Enter подтверждает, ESC отменяет.
// Набранный текст оставляет строки, в текст которых он входит без учёта регистра.
func (u *UI) Select(items []SelectItem) ([]int, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil, fmt.Errorf("interactive input is not available")
	}
	return runSelect(items)
}

type picker struct {
	items    []SelectItem
	on       []bool
	filter   string
	cursor   int
	terminal bool
}

func (p *picker) visible() []int {
	q := strings.ToLower(p.filter)
	idx := make([]int, 0, len(p.items))
	for i, it := range p.items {
		if q == "" || strings.Contains(strings.ToLower(it.Text), q) {
			idx = append(idx, i)
		}
	}
	return idx
}

func (p *picker) clamp() {
	vis := p.visible()
	if len(vis) == 0 {
		p.cursor = 0
		return
	}
	if p.cursor >= len(vis) {
		p.cursor = len(vis) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *picker) onKey(code keys.KeyCode, text string) (stop, cancel bool) {
	switch code {
	case keys.Escape, keys.CtrlC:
		return true, true
	case keys.Enter:
		return true, false
	case keys.Up:
		vis := p.visible()
		if len(vis) == 0 {
			return false, false
		}
		if p.cursor <= 0 {
			p.cursor = len(vis) - 1
		} else {
			p.cursor--
		}
	case keys.Down:
		vis := p.visible()
		if len(vis) == 0 {
			return false, false
		}
		if p.cursor >= len(vis)-1 {
			p.cursor = 0
		} else {
			p.cursor++
		}
	case keys.Space:
		vis := p.visible()
		if len(vis) == 0 || p.cursor < 0 || p.cursor >= len(vis) {
			return false, false
		}
		i := vis[p.cursor]
		p.on[i] = !p.on[i]
	case keys.Backspace:
		r := []rune(p.filter)
		if len(r) > 0 {
			p.filter = string(r[:len(r)-1])
		}
		p.clamp()
	case keys.RuneKey:
		p.filter += text
		p.clamp()
	}
	return false, false
}

func (p *picker) selected() []int {
	out := make([]int, 0)
	for i, on := range p.on {
		if on {
			out = append(out, i)
		}
	}
	return out
}

func (p *picker) render() string {
	var b strings.Builder
	b.WriteString("Select repositories")
	if p.filter != "" {
		b.WriteString(" / ")
		b.WriteString(p.filter)
	}
	b.WriteByte('\n')
	vis := p.visible()
	if len(vis) == 0 {
		b.WriteString("(no matches)\n")
		return b.String()
	}
	for n, i := range vis {
		mark := "[ ]"
		if p.on[i] {
			mark = "[x]"
		}
		cursor := " "
		if n == p.cursor {
			cursor = ">"
		}
		label := p.items[i].Text
		if p.terminal {
			label = osc8(label, p.items[i].URL)
		}
		fmt.Fprintf(&b, "%s %s %s\n", cursor, mark, label)
	}
	return b.String()
}

func osc8(text, rawURL string) string {
	if rawURL == "" || strings.ContainsAny(text, "\a\x1b") || strings.ContainsAny(rawURL, "\a\x1b") {
		return text
	}
	return "\033]8;;" + rawURL + "\033\\" + text + "\033]8;;\033\\"
}

func runSelect(items []SelectItem) ([]int, error) {
	p := &picker{items: items, on: make([]bool, len(items)), terminal: true}
	area, err := pterm.DefaultArea.Start(p.render())
	if err != nil {
		return nil, err
	}
	defer func() { _ = area.Stop() }()
	var cancelled bool
	err = keyboard.Listen(func(key keys.Key) (bool, error) {
		text := ""
		if key.Code == keys.RuneKey {
			text = string(key.Runes)
		}
		stop, cancel := p.onKey(key.Code, text)
		cancelled = cancel
		if !stop {
			area.Update(p.render())
		}
		return stop, nil
	})
	if err != nil {
		return nil, err
	}
	if cancelled {
		return nil, ErrSelectCancelled
	}
	return p.selected(), nil
}
