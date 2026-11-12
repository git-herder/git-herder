// Критерии SCM-отбора после резолва argv и профиля.
package scm

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Snowbridge/git-herder/internal/resolved"
)

var qpQnPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// TagFilter — условие --qt.
type TagFilter struct {
	Name   string
	Absent bool
}

// AttrFilter — условие --qa.
type AttrFilter struct {
	Key   string
	Op    string
	Value string
}

// SearchPair — одна пара key=value в --scm-search.
type SearchPair struct {
	Key   string
	Value string
}

// Criteria — итог резолва переносимых и extension-параметров отбора.
type Criteria struct {
	Paths              []string
	Names              []string
	Tags               []TagFilter
	Attrs              []AttrFilter
	Search             bool
	SearchPairs        []SearchPair
	Scope              resolved.SelectionScope
	IncludeInactive    bool
	IncludeInactiveSet bool
	OnLimit            string
	OnLimitSet         bool
}

// HasCriterion — задан хотя бы один критерий отбора (не scope и не модификаторы).
func (c Criteria) HasCriterion() bool {
	if c.Search {
		return true
	}
	return len(c.Paths)+len(c.Names)+len(c.Tags)+len(c.Attrs) > 0
}

// GitHubHasQueryFragment — есть фрагмент для Search q (qp, qn или qa с =).
func (c Criteria) GitHubHasQueryFragment() bool {
	if len(c.Paths) > 0 || len(c.Names) > 0 {
		return true
	}
	for _, a := range c.Attrs {
		if a.Op == "=" {
			return true
		}
	}
	return false
}

// PresenceTags — имена тегов «наличие».
func (c Criteria) PresenceTags() []string {
	var out []string
	for _, t := range c.Tags {
		if !t.Absent {
			out = append(out, t.Name)
		}
	}
	return out
}

// EqualAttrs — условия --qa с оператором =.
func (c Criteria) EqualAttrs() []NameValue {
	var out []NameValue
	for _, a := range c.Attrs {
		if a.Op == "=" {
			out = append(out, NameValue{Name: a.Key, Value: a.Value})
		}
	}
	return out
}

// ParseTag разбирает одно значение --qt.
func ParseTag(raw string) (TagFilter, error) {
	if strings.ContainsAny(raw, "<>") {
		return TagFilter{}, fmt.Errorf("invalid --qt %q", raw)
	}
	lower := strings.ToLower(raw)
	const suffix = "=null"
	if strings.HasSuffix(lower, suffix) {
		name := raw[:len(raw)-len(suffix)]
		if name == "" {
			return TagFilter{}, fmt.Errorf("invalid --qt %q", raw)
		}
		return TagFilter{Name: name, Absent: true}, nil
	}
	if strings.Contains(raw, "=") {
		return TagFilter{}, fmt.Errorf("invalid --qt %q", raw)
	}
	if raw == "" {
		return TagFilter{}, fmt.Errorf("invalid --qt %q", raw)
	}
	return TagFilter{Name: raw}, nil
}

// ParseAttr разбирает одно значение --qa.
func ParseAttr(raw string) (AttrFilter, error) {
	ops := []string{"<=", ">=", "!=", "<", ">", "="}
	idx := -1
	op := ""
	for i := 0; i < len(raw); i++ {
		for _, candidate := range ops {
			if strings.HasPrefix(raw[i:], candidate) {
				idx = i
				op = candidate
				break
			}
		}
		if idx >= 0 {
			break
		}
	}
	if idx < 0 {
		return AttrFilter{}, fmt.Errorf("invalid --qa %q", raw)
	}
	key := raw[:idx]
	if key == "" {
		return AttrFilter{}, fmt.Errorf("invalid --qa %q", raw)
	}
	return AttrFilter{Key: key, Op: op, Value: raw[idx+len(op):]}, nil
}

// ParseSearch разбирает строку scm-search в пары key=value (последняя побеждает).
func ParseSearch(raw string) ([]SearchPair, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid --scm-search %q", raw)
	}
	var order []string
	vals := map[string]string{}
	for _, f := range fields {
		key, val, ok := strings.Cut(f, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --scm-search token %q", f)
		}
		if _, seen := vals[key]; !seen {
			order = append(order, key)
		}
		vals[key] = val
	}
	out := make([]SearchPair, 0, len(order))
	for _, k := range order {
		out = append(out, SearchPair{Key: k, Value: vals[k]})
	}
	return out, nil
}

// ValidateShape проверяет контракт до запросов к SCM.
// Пустой набор критериев и отсутствие фрагментов q GitHub сюда не входят.
func (c *Criteria) ValidateShape(up resolved.Upstream) error {
	if c.Search && (len(c.Paths)+len(c.Names)+len(c.Tags)+len(c.Attrs) > 0) {
		return fmt.Errorf("scm-search cannot be combined with --qp, --qn, --qt, or --qa")
	}
	switch up {
	case resolved.UpstreamGitHub:
		if c.IncludeInactiveSet {
			return fmt.Errorf("include-inactive is not valid for upstream github")
		}
		if c.OnLimitSet && c.OnLimit != "await" && c.OnLimit != "abort" {
			return fmt.Errorf("invalid --on-limit-exceeded %q", c.OnLimit)
		}
		if !c.OnLimitSet {
			c.OnLimit = "await"
		}
		if !c.Search {
			for _, p := range c.Paths {
				if !qpQnPattern.MatchString(p) {
					return fmt.Errorf("invalid --qp %q", p)
				}
			}
			for _, n := range c.Names {
				if !qpQnPattern.MatchString(n) {
					return fmt.Errorf("invalid --qn %q", n)
				}
			}
		}
	case resolved.UpstreamGitLab:
		if c.OnLimitSet {
			return fmt.Errorf("on-limit-exceeded is not valid for upstream gitlab")
		}
	default:
		return fmt.Errorf("upstream is required")
	}
	if c.Search {
		pairs, err := normalizeSearch(up, c.SearchPairs)
		if err != nil {
			return err
		}
		c.SearchPairs = pairs
	}
	return nil
}

// ValidateGitHubFragments — ошибка до HTTP, если в режиме --q* не из чего собрать q.
func (c Criteria) ValidateGitHubFragments(up resolved.Upstream) error {
	if up != resolved.UpstreamGitHub || c.Search {
		return nil
	}
	if c.GitHubHasQueryFragment() {
		return nil
	}
	return fmt.Errorf("github search query has no server-side fragments: set --qp, --qn, or --qa with =")
}

func normalizeSearch(up resolved.Upstream, pairs []SearchPair) ([]SearchPair, error) {
	switch up {
	case resolved.UpstreamGitHub:
		q, ok := pairValue(pairs, "q")
		if !ok || strings.TrimSpace(q) == "" {
			return nil, fmt.Errorf("invalid --scm-search: missing q")
		}
		return pairs, nil
	case resolved.UpstreamGitLab:
		scope, ok := pairValue(pairs, "scope")
		if !ok {
			return append(pairs, SearchPair{Key: "scope", Value: "projects"}), nil
		}
		if !strings.EqualFold(scope, "projects") {
			return nil, fmt.Errorf("invalid --scm-search scope %q", scope)
		}
		out := make([]SearchPair, len(pairs))
		copy(out, pairs)
		for i := range out {
			if out[i].Key == "scope" {
				out[i].Value = "projects"
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("upstream is required")
	}
}

func pairValue(pairs []SearchPair, key string) (string, bool) {
	var (
		v  string
		ok bool
	)
	for _, p := range pairs {
		if p.Key == key {
			v = p.Value
			ok = true
		}
	}
	return v, ok
}

// PublicSaaSHost — хост в точности github.com или gitlab.com.
func PublicSaaSHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexAny(h, "/:"); i >= 0 {
		// отрезать путь; порт у github.com:443 тоже отбрасываем
		hostOnly := h
		if slash := strings.IndexByte(h, '/'); slash >= 0 {
			hostOnly = h[:slash]
		}
		if colon := strings.IndexByte(hostOnly, ':'); colon >= 0 {
			hostOnly = hostOnly[:colon]
		}
		h = hostOnly
	}
	return h == "github.com" || h == "gitlab.com"
}
