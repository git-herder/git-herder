// Пакет resolved: нормализованные значения после резолва argv и профиля.
package resolved

import (
	"fmt"
	"strings"
)

// Upstream — SCM-платформа после резолва.
type Upstream int

const (
	UpstreamUnset Upstream = iota
	UpstreamGitHub
	UpstreamGitLab
)

// ParseUpstream разбирает строку upstream (регистр не важен).
func ParseUpstream(s string) (Upstream, error) {
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
		return "https://gitlab.com/api/v4"
	default:
		return ""
	}
}
