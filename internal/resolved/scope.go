package resolved

import (
	"fmt"
	"strings"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// SelectionScope — продуктовый контур перечисления (hub scope).
type SelectionScope int

const (
	SelectionScopeAffiliated SelectionScope = iota
	SelectionScopeAll
)

func (s SelectionScope) String() string {
	if s == SelectionScopeAll {
		return "all"
	}
	return "affiliated"
}

// ParseSelectionScope разбирает значение scope.
func ParseSelectionScope(s string) (scope SelectionScope, err error) {
	leave := logging.Enter("s", s)
	defer func() { leave("scope", scope, "err", err) }()
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "affiliated" {
		return SelectionScopeAffiliated, nil
	}
	switch s {
	case "all":
		return SelectionScopeAll, nil
	default:
		return SelectionScopeAffiliated, fmt.Errorf("invalid scope %q", s)
	}
}
