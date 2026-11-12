package resolved

import (
	"fmt"
	"strings"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// OnError — политика после исчерпания повторов.
type OnError int

const (
	OnErrorContinue OnError = iota
	OnErrorAbort
)

func (p OnError) String() string {
	if p == OnErrorAbort {
		return "abort"
	}
	return "continue"
}

// ParseOnError разбирает --on-error.
func ParseOnError(s string) (policy OnError, err error) {
	leave := logging.Enter("s", s)
	defer func() { leave("onError", policy, "err", err) }()
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
