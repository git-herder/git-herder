// Клиентская фильтрация карточек после обогащения.
package selection

import (
	"regexp"
	"strings"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// Filter применяет клиентские условия --q* текущего upstream.
func Filter(up resolved.Upstream, cards []card.Card, crit scm.Criteria) (out []card.Card) {
	leave := logging.Enter("upstream", up, "in", len(cards), "search", crit.Search)
	defer func() { leave("out", len(out)) }()
	if crit.Search {
		return cards
	}
	out = make([]card.Card, 0, len(cards))
	for _, c := range cards {
		if reason := dropReason(up, c, crit); reason != "" {
			logging.Debug("filter drop", "fullName", c.FullName(), "reason", reason)
			continue
		}
		out = append(out, c)
	}
	return out
}

func matchCard(up resolved.Upstream, c card.Card, crit scm.Criteria) bool {
	return dropReason(up, c, crit) == ""
}

func dropReason(up resolved.Upstream, c card.Card, crit scm.Criteria) string {
	switch up {
	case resolved.UpstreamGitLab:
		for _, p := range crit.Paths {
			if !matchText(p, c.Path) {
				return "path"
			}
		}
		for _, n := range crit.Names {
			if !matchText(n, c.Name) {
				return "name"
			}
		}
		for _, t := range crit.Tags {
			if t.Absent && hasLabel(c.Labels, t.Name) {
				return "tag"
			}
		}
	case resolved.UpstreamGitHub:
		for _, t := range crit.Tags {
			present := hasLabel(c.Labels, t.Name)
			if t.Absent && present {
				return "tag"
			}
			if !t.Absent && !present {
				return "tag"
			}
		}
	}
	for _, a := range crit.Attrs {
		if a.Op == "=" {
			continue
		}
		if !matchAttr(c.Properties, a) {
			return "attr"
		}
	}
	return ""
}

func matchText(pattern, value string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(value, pattern)
	}
	return re.MatchString(value)
}

func hasLabel(labels []string, name string) bool {
	for _, l := range labels {
		if l == name {
			return true
		}
	}
	return false
}

func matchAttr(props map[string]any, a scm.AttrFilter) bool {
	v, ok := props[a.Key]
	if !ok {
		return false
	}
	left := card.Canonical(v)
	right := a.Value
	switch a.Op {
	case "!=":
		return left != right
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}
