// Пакет conn: конфигурация подключения к SCM (не только listing).
package conn

import (
	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
)

// Config — подключение к SCM после резолва Invocation.
type Config struct {
	Upstream        resolved.Upstream
	Host            string
	Token           string
	SelectionScope  resolved.SelectionScope
	IncludeInactive bool
	PerPage         int
}

// FromGlobals строит conn из Globals и параметров команды.
func FromGlobals(g resolved.Globals, perPage int, includeInactive bool) Config {
	if perPage <= 0 {
		perPage = scm.DefaultPerPage
	}
	return Config{
		Upstream:        g.Upstream,
		Host:            g.Host,
		Token:           g.Token,
		SelectionScope:  g.SelectionScope,
		IncludeInactive: includeInactive,
		PerPage:         perPage,
	}
}
