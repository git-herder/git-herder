// Пакет report: RepositoriesReportBuilder — stdout TSV и JSON-файл.
package report

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// WriteStdout пишет по строке на репозиторий: полное имя, таб, url.
func WriteStdout(cards []card.Card) error {
	sorted := sortedCopy(cards)
	var buf bytes.Buffer
	for _, c := range sorted {
		buf.WriteString(c.FullName())
		buf.WriteByte('\t')
		buf.WriteString(c.URL)
		buf.WriteByte('\n')
	}
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// WriteJSON записывает один JSON-массив карточек.
func WriteJSON(path string, cards []card.Card) error {
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
