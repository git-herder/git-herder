package gitlab

import (
	"encoding/json"
	"strings"

	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/tstamp"
)

func coerceText(text string) any {
	switch strings.ToLower(text) {
	case "true":
		return true
	case "false":
		return false
	}
	if n, ok := card.NormalizeNumber(text); ok {
		return json.Number(n)
	}
	if tstamp.IsDataTimestamp(text) {
		return text
	}
	return text
}
