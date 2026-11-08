package resolved

import (
	"fmt"
	"strings"
)

// OnError — политика после исчерпания повторов.
type OnError int

const (
	OnErrorContinue OnError = iota
	OnErrorAbort
)

// ParseOnError разбирает --on-error.
func ParseOnError(s string) (OnError, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return OnErrorContinue, nil
	}
	switch s {
	case "continue":
		return OnErrorContinue, nil
	case "abort":
		return OnErrorAbort, nil
	default:
		return OnErrorContinue, fmt.Errorf("invalid --on-error %q", s)
	}
}
