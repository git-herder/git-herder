// Клиентская фильтрация карточек после обогащения.
package selection

import (
	"regexp"
	"strings"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// Filter применяет клиентские условия --q* текущего upstream.
func Filter(up resolved.Upstream, cards []card.Card, crit scm.Criteria) []card.Card {
	if crit.Search {
		return cards
	}
	out := make([]card.Card, 0, len(cards))
	for _, c := range cards {
		if matchCard(up, c, crit) {
			out = append(out, c)
		}
	}
	return out
}

func matchCard(up resolved.Upstream, c card.Card, crit scm.Criteria) bool {
	switch up {
	case resolved.UpstreamGitLab:
		for _, p := range crit.Paths {
			if !matchText(p, c.Path) {
				return false
			}
		}
		for _, n := range crit.Names {
			if !matchText(n, c.Name) {
				return false
			}
		}
		for _, t := range crit.Tags {
			if t.Absent && hasLabel(c.Labels, t.Name) {
				return false
			}
		}
	case resolved.UpstreamGitHub:
		for _, t := range crit.Tags {
			present := hasLabel(c.Labels, t.Name)
			if t.Absent && present {
				return false
			}
			if !t.Absent && !present {
				return false
			}
		}
	}
	for _, a := range crit.Attrs {
		if a.Op == "=" {
			continue
		}
		if !matchAttr(c.Properties, a) {
			return false
		}
	}
	return true
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
