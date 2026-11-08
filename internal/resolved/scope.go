package resolved

import (
	"fmt"
	"strings"
)

// SelectionScope — продуктовый контур перечисления (hub scope).
type SelectionScope int

const (
	SelectionScopeAffiliated SelectionScope = iota
	SelectionScopeAll
)

// ParseSelectionScope разбирает значение scope.
func ParseSelectionScope(s string) (SelectionScope, error) {
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
