package selection

import (
	"os"

	"github.com/pterm/pterm"
)

// Meter — прогресс страниц листинга и пар обогащения.
type Meter interface {
	Start(title string, total int)
	Advance()
	Stop()
}

// NopMeter ничего не рисует.
type NopMeter struct{}

func (NopMeter) Start(string, int) {}
func (NopMeter) Advance()          {}
func (NopMeter) Stop()             {}

// PtermMeter рисует progress bar в stderr.
type PtermMeter struct {
	pb *pterm.ProgressbarPrinter
}

func (m *PtermMeter) Start(title string, total int) {
	if total < 1 {
		total = 1
	}
	printer := pterm.DefaultProgressbar
	pb, err := printer.WithTotal(total).WithTitle(title).WithWriter(os.Stderr).Start()
	if err != nil || pb == nil {
		return
	}
	m.pb = pb
}

func (m *PtermMeter) Advance() {
	if m != nil && m.pb != nil {
		m.pb.Increment()
	}
}

func (m *PtermMeter) Stop() {
	if m != nil && m.pb != nil {
		_, _ = m.pb.Stop()
		m.pb = nil
	}
}
