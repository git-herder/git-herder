// Пакет report: RepositoriesReportBuilder — stdout TSV и JSON-файл.
package report

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/scm/card"
)

// WriteStdout пишет по строке на репозиторий.
// Терминал — имя-ссылка OSC 8; пайп и файл — полное имя, таб, url.
func WriteStdout(cards []card.Card) (err error) {
	leave := logging.Enter("count", len(cards))
	defer func() { leave("err", err) }()
	sorted := sortedCopy(cards)
	terminal := term.IsTerminal(int(os.Stdout.Fd()))
	var buf bytes.Buffer
	for _, c := range sorted {
		buf.WriteString(formatStdoutLine(c.FullName(), c.URL, terminal))
	}
	_, err = os.Stdout.Write(buf.Bytes())
	return err
}

func formatStdoutLine(name, rawURL string, terminal bool) string {
	if !terminal || rawURL == "" || strings.ContainsAny(name, "\a\x1b") || strings.ContainsAny(rawURL, "\a\x1b") {
		return name + "\t" + rawURL + "\n"
	}
	return "\033]8;;" + rawURL + "\033\\" + name + "\033]8;;\033\\\n"
}

// WriteJSON записывает один JSON-массив карточек.
func WriteJSON(path string, cards []card.Card) (err error) {
	leave := logging.Enter("path", path, "count", len(cards))
	defer func() { leave("err", err) }()
	sorted := sortedCopy(cards)
	payload := make([]card.Card, len(sorted))
	for i := range sorted {
		sorted[i].Normalize()
		payload[i] = sorted[i]
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func sortedCopy(cards []card.Card) []card.Card {
	out := append([]card.Card(nil), cards...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].FullName() < out[j].FullName()
	})
	return out
}
