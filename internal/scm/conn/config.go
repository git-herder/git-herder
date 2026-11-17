// Пакет conn: конфигурация подключения к SCM (не только listing).
package conn

import (
	"time"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
)

// Config — подключение к SCM после резолва Invocation.
type Config struct {
	Upstream        resolved.Upstream
	Host            string
	Token           string
	SelectionScope  resolved.SelectionScope
	IncludeInactive bool
	PerPage         int
	OnLimit         string
	OnQuotaWait     func(time.Time)
}

// FromGlobals строит conn из Globals и параметров команды.
func FromGlobals(g resolved.Globals, perPage int, includeInactive bool) (cfg Config) {
	leave := logging.Enter("upstream", g.Upstream, "host", g.Host, "token", g.Token, "perPage", perPage, "includeInactive", includeInactive)
	defer func() {
		leave("upstream", cfg.Upstream, "host", cfg.Host, "token", cfg.Token, "perPage", cfg.PerPage)
	}()
	if perPage <= 0 {
		perPage = scm.DefaultPerPage
	}
	cfg = Config{
		Upstream:        g.Upstream,
		Host:            g.Host,
		Token:           g.Token,
		SelectionScope:  g.SelectionScope,
		IncludeInactive: includeInactive,
		PerPage:         perPage,
	}
	return cfg
}
