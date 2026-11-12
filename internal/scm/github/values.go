package github

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/Snowbridge/git-herder/internal/tstamp"
)

// coerceGitHubProperty переводит элемент value из custom properties API в scalar карточки.
// ok == false означает null: ключ не добавляется.
func coerceGitHubProperty(raw json.RawMessage) (any, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	if string(raw) == "true" {
		return true, true
	}
	if string(raw) == "false" {
		return false, true
	}
	if raw[0] != '"' && raw[0] != '[' && raw[0] != '{' {
		if n, ok := normalizeJSONNumber(string(raw)); ok {
			return json.Number(n), true
		}
		return string(raw), true
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return string(raw), true
		}
		if tstamp.IsDataTimestamp(s) {
			return s, true
		}
		return s, true
	}
	if raw[0] == '[' {
		var buf bytes.Buffer
		if err := json.Compact(&buf, raw); err != nil {
			return string(raw), true
		}
		return buf.String(), true
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw), true
	}
	return buf.String(), true
}

func normalizeJSONNumber(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "eE") {
		return "", false
	}
	return normalizePlainNumber(s)
}

func normalizePlainNumber(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	sign := ""
	if s[0] == '-' {
		sign = "-"
		s = s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	if intPart == "" || !allDigits(intPart) {
		return "", false
	}
	if len(intPart) > 1 && intPart[0] == '0' {
		return "", false
	}
	if !hasFrac {
		return sign + intPart, true
	}
	if frac == "" || !allDigits(frac) {
		return "", false
	}
	zero := true
	for _, c := range frac {
		if c != '0' {
			zero = false
			break
		}
	}
	if zero {
		return sign + intPart, true
	}
	return sign + intPart + "." + frac, true
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
