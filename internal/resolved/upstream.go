// Пакет resolved: нормализованные значения после резолва argv и профиля.
package resolved

import (
	"fmt"
	"strings"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// Upstream — SCM-платформа после резолва.
type Upstream int

const (
	UpstreamUnset Upstream = iota
	UpstreamGitHub
	UpstreamGitLab
)

func (u Upstream) String() string {
	switch u {
	case UpstreamGitHub:
		return "github"
	case UpstreamGitLab:
		return "gitlab"
	default:
		return "unset"
	}
}

// ParseUpstream разбирает строку upstream (регистр не важен).
func ParseUpstream(s string) (u Upstream, err error) {
	leave := logging.Enter("s", s)
	defer func() { leave("upstream", u, "err", err) }()
	s = strings.TrimSpace(s)
	if s == "" {
		return UpstreamUnset, nil
	}
	switch strings.ToLower(s) {
	case "github":
		return UpstreamGitHub, nil
	case "gitlab":
		return UpstreamGitLab, nil
	default:
		return UpstreamUnset, fmt.Errorf("invalid --upstream %q", s)
	}
}

// DefaultHost — база REST API по умолчанию, если host не задан.
func DefaultHost(u Upstream) string {
	switch u {
	case UpstreamGitHub:
		return "https://api.github.com"
	case UpstreamGitLab:
		return "https://gitlab.com"
	default:
		return ""
	}
}
