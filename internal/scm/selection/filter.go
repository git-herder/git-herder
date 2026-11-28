// Клиентская фильтрация карточек: полный проход либо ранняя и поздняя фазы.
package selection

import (
	"regexp"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
)

type fieldPhase int

const (
	phaseAll fieldPhase = iota
	phaseEarly
	phaseLate
)

// Filter применяет все клиентские условия текущего upstream.
// Так проходит отбор, когда активных заданий обогащения нет.
func Filter(up resolved.Upstream, cards []card.Card, crit scm.Criteria) (out []card.Card) {
	return filterPhase(up, cards, crit, nil, phaseAll)
}

func filterEarly(up resolved.Upstream, cards []card.Card, crit scm.Criteria, jobs []scm.Enricher) []card.Card {
	return filterPhase(up, cards, crit, pendingFields(jobs), phaseEarly)
}

func filterLate(up resolved.Upstream, cards []card.Card, crit scm.Criteria, jobs []scm.Enricher) []card.Card {
	return filterPhase(up, cards, crit, pendingFields(jobs), phaseLate)
}

func filterPhase(up resolved.Upstream, cards []card.Card, crit scm.Criteria, pending map[string]bool, ph fieldPhase) (out []card.Card) {
	leave := logging.Enter("upstream", up, "in", len(cards), "search", crit.Search, "phase", ph)
	defer func() { leave("out", len(out)) }()
	if crit.Search {
		return cards
	}
	out = make([]card.Card, 0, len(cards))
	for _, c := range cards {
		if reason := dropReason(up, c, crit, pending, ph); reason != "" {
			logging.Debug("filter drop", "fullName", c.FullName(), "reason", reason)
			continue
		}
		out = append(out, c)
	}
	return out
}

func pendingFields(jobs []scm.Enricher) map[string]bool {
	out := map[string]bool{}
	for _, job := range jobs {
		if job != nil {
			out[job.ID()] = true
		}
	}
	return out
}

// fieldOn — критерий этой фазы. Поле без задания окончательно после list (ранняя фаза).
func fieldOn(id string, pending map[string]bool, ph fieldPhase) bool {
	on := pending[id]
	switch ph {
	case phaseLate:
		return on
	case phaseEarly:
		return !on
	default:
		return true
	}
}

func dropReason(up resolved.Upstream, c card.Card, crit scm.Criteria, pending map[string]bool, ph fieldPhase) string {
	if ph != phaseLate && up == resolved.UpstreamGitLab {
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
	}
	if fieldOn("labels", pending, ph) {
		switch up {
		case resolved.UpstreamGitLab:
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
	}
	if fieldOn("properties", pending, ph) {
		for _, a := range crit.Attrs {
			if a.Op == "=" {
				continue
			}
			if !matchAttr(c.Properties, a) {
				return "attr"
			}
		}
	}
	if fieldOn("branches", pending, ph) && !matchBranches(c.Branches, crit.Branches) {
		return "branch"
	}
	return ""
}

func matchBranches(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, name := range have {
		set[name] = struct{}{}
	}
	for _, name := range want {
		if _, ok := set[name]; !ok {
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
