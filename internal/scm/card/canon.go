package card

import (
	"encoding/json"
	"strings"
)

// Canonical — строка scalar для лексикографического сравнения --qa.
func Canonical(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case json.Number:
		if n, ok := NormalizeNumber(x.String()); ok {
			return n
		}
		return x.String()
	case string:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// NormalizeNumber принимает десятичное число без экспоненты.
// Целые с дробной частью из нулей сворачиваются (1.0 → 1).
func NormalizeNumber(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "eE+") {
		return "", false
	}
	sign := ""
	if s[0] == '-' {
		sign = "-"
		s = s[1:]
	}
	if s == "" || s[0] == '.' {
		return "", false
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	if intPart == "" || !digits(intPart) {
		return "", false
	}
	if len(intPart) > 1 && intPart[0] == '0' {
		return "", false
	}
	if !hasFrac {
		return sign + intPart, true
	}
	if frac == "" || !digits(frac) {
		return "", false
	}
	allZero := true
	for _, c := range frac {
		if c != '0' {
			allZero = false
			break
		}
	}
	if allZero {
		return sign + intPart, true
	}
	return sign + intPart + "." + frac, true
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
