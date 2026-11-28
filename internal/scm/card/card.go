// Пакет card: нормализованная карточка SCM-репозитория по platform/scm-repository-card.
package card

import (
	"encoding/json"
	"strconv"
)

// UID — идентификатор репозитория: строка (GitHub) или число (GitLab).
type UID struct {
	text  string
	num   int64
	isNum bool
}

// TextUID создаёт строковый uid.
func TextUID(s string) UID { return UID{text: s} }

// NumberUID создаёт числовой uid.
func NumberUID(n int64) UID { return UID{num: n, isNum: true} }

// String возвращает десятичную запись числа или исходную строку.
func (u UID) String() string {
	if u.isNum {
		return strconv.FormatInt(u.num, 10)
	}
	return u.text
}

// Number возвращает числовой uid GitLab.
func (u UID) Number() (int64, bool) {
	if !u.isNum {
		return 0, false
	}
	return u.num, true
}

// MarshalJSON кодирует uid строкой или числом.
func (u UID) MarshalJSON() ([]byte, error) {
	if u.isNum {
		return json.Marshal(u.num)
	}
	return json.Marshal(u.text)
}

// UnmarshalJSON принимает JSON string или number.
func (u *UID) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*u = TextUID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	v, err := n.Int64()
	if err != nil {
		return err
	}
	*u = NumberUID(v)
	return nil
}

// Card — обобщённая запись репозитория.
type Card struct {
	UID         UID            `json:"uid"`
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	URL         string         `json:"url"`
	Description string         `json:"description"`
	Labels      []string       `json:"labels"`
	Properties  map[string]any `json:"properties"`
	Branches    []string       `json:"branches"`
}

// EmptyProperties возвращает пустой объект properties.
func EmptyProperties() map[string]any {
	return map[string]any{}
}

// FullName — колонка «полное имя» для stdout: path/name или name.
func (c Card) FullName() string {
	if c.Path == "" {
		return c.Name
	}
	return c.Path + "/" + c.Name
}

// SetLabels полностью заменяет метки.
func (c *Card) SetLabels(labels []string) {
	if labels == nil {
		labels = []string{}
	}
	c.Labels = labels
}

// SetProperties полностью заменяет properties.
func (c *Card) SetProperties(props map[string]any) {
	if props == nil {
		props = EmptyProperties()
	}
	c.Properties = props
}

// SetBranches полностью заменяет имена веток.
func (c *Card) SetBranches(names []string) {
	if names == nil {
		names = []string{}
	}
	c.Branches = append([]string(nil), names...)
}

// Normalize заполняет nil-срезы, чтобы JSON не содержал null.
func (c *Card) Normalize() {
	if c.Labels == nil {
		c.Labels = []string{}
	}
	if c.Properties == nil {
		c.Properties = EmptyProperties()
	}
	if c.Branches == nil {
		c.Branches = []string{}
	}
}

// Set — набор карточек с заменой полей обогащения.
type Set struct {
	Items []Card
}

// Append добавляет карточку.
func (s *Set) Append(c Card) {
	c.Normalize()
	s.Items = append(s.Items, c)
}

// ReplaceLabels полностью заменяет labels карточки.
func (s *Set) ReplaceLabels(i int, labels []string) {
	if labels == nil {
		labels = []string{}
	}
	s.Items[i].Labels = labels
}

// ReplaceProperties полностью заменяет properties карточки.
func (s *Set) ReplaceProperties(i int, props map[string]any) {
	if props == nil {
		props = EmptyProperties()
	}
	s.Items[i].Properties = props
}

// Filter оставляет карточки, для которых pred истинен.
func (s Set) Filter(pred func(Card) bool) Set {
	out := Set{}
	for _, c := range s.Items {
		if pred(c) {
			out.Append(c)
		}
	}
	return out
}
