package console

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// GitStep — строка хода долгого шага git на stderr.
// nil-приёмник ничего не пишет: тесты передают nil.
type GitStep struct {
	out      io.Writer
	terminal bool
	done     int
	total    int

	now      func() time.Time
	interval time.Duration
	// ticks заменяет таймер в тесте. nil — тик раз в interval.
	ticks   chan struct{}
	onPaint func()

	mu      sync.Mutex
	active  bool
	step    string
	label   string
	attempt int
	started time.Time
	stop    chan struct{}
	stopped chan struct{}
}

// NewGitStep пишет строку в w. terminal — stderr является терминалом.
// total — сколько объектов поставлено в очередь.
func NewGitStep(w io.Writer, terminal bool, total int) *GitStep {
	if total < 0 {
		total = 0
	}
	return &GitStep{
		out:      w,
		terminal: terminal,
		total:    total,
		now:      time.Now,
		interval: time.Second,
	}
}

// SetTotal задаёт размер очереди, когда он становится известен.
func (g *GitStep) SetTotal(n int) {
	if g == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	g.mu.Lock()
	g.total = n
	g.mu.Unlock()
}

// Start показывает шаг. На терминале строка обновляется раз в секунду.
// Без терминала печатается одна строка с 0:00.
func (g *GitStep) Start(step, label string, attempt int) {
	if g == nil || g.out == nil {
		return
	}
	g.Stop()
	g.mu.Lock()
	g.step = step
	g.label = label
	g.attempt = attempt
	g.started = g.clock()
	g.active = true
	elapsed := time.Duration(0)
	if g.terminal {
		elapsed = g.clock().Sub(g.started)
	}
	g.writeLocked(g.line(elapsed))
	if !g.terminal {
		g.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	stopped := make(chan struct{})
	g.stop = stop
	g.stopped = stopped
	g.mu.Unlock()
	go g.loop(stop, stopped)
}

// Stop останавливает обновление и оставляет последнюю строку на экране.
func (g *GitStep) Stop() {
	if g == nil {
		return
	}
	g.mu.Lock()
	stop := g.stop
	stopped := g.stopped
	g.stop = nil
	g.stopped = nil
	g.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-stopped
}

// Clear стирает строку на терминале. Без терминала ничего не пишет.
func (g *GitStep) Clear() {
	if g == nil {
		return
	}
	g.Stop()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.terminal && g.active && g.out != nil {
		fmt.Fprint(g.out, "\r\033[K")
	}
	g.active = false
}

// MarkDone учитывает объект, который уже получил итоговую строку.
func (g *GitStep) MarkDone() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.done++
	g.mu.Unlock()
}

// Result стирает ход, печатает итог и увеличивает счётчик сделанного.
func (g *GitStep) Result(print func(string), line string) {
	if g == nil {
		if print != nil {
			print(line)
		}
		return
	}
	g.Clear()
	if print != nil {
		print(line)
	}
	g.MarkDone()
}

func (g *GitStep) loop(stop <-chan struct{}, stopped chan struct{}) {
	defer close(stopped)
	if g.ticks != nil {
		for {
			select {
			case <-stop:
				return
			case <-g.ticks:
				g.repaint()
			}
		}
	}
	interval := g.interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			g.repaint()
		}
	}
}

func (g *GitStep) repaint() {
	g.mu.Lock()
	if g.active && g.terminal {
		g.writeLocked(g.line(g.clock().Sub(g.started)))
	}
	hook := g.onPaint
	g.mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (g *GitStep) line(elapsed time.Duration) string {
	return formatGitStep(g.step, g.done, g.total, g.label, g.attempt, elapsed)
}

func (g *GitStep) writeLocked(text string) {
	if g.out == nil {
		return
	}
	if g.terminal {
		fmt.Fprintf(g.out, "\r%s\033[K", text)
		return
	}
	fmt.Fprintln(g.out, text)
}

func (g *GitStep) clock() time.Time {
	if g.now == nil {
		return time.Now()
	}
	return g.now()
}

func formatGitStep(step string, done, total int, label string, attempt int, elapsed time.Duration) string {
	secs := int(elapsed / time.Second)
	if secs < 0 {
		secs = 0
	}
	mins := secs / 60
	rem := secs % 60
	if attempt >= 2 {
		return fmt.Sprintf("%s %d/%d %s attempt %d %d:%02d", step, done, total, label, attempt, mins, rem)
	}
	return fmt.Sprintf("%s %d/%d %s %d:%02d", step, done, total, label, mins, rem)
}
