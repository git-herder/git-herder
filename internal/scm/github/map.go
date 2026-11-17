// Маппинг элемента GitHub Search API в card.Card (только list, без enrichment).
package github

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/git-herder/git-herder/internal/scm/card"
)

// ErrMissingURL — в элементе list нет непустого URL страницы репозитория.
var ErrMissingURL = errors.New("repository list item is missing url")

type searchItem struct {
	FullName    string   `json:"full_name"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	HTMLURL     *string  `json:"html_url"`
	Topics      []string `json:"topics"`
}

// MapSearchItem строит карточку из элемента items ответа GET /search/repositories.
func MapSearchItem(raw json.RawMessage) (card.Card, error) {
	var item searchItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return card.Card{}, err
	}
	desc := ""
	if item.Description != nil {
		desc = *item.Description
	}
	if item.HTMLURL == nil || strings.TrimSpace(*item.HTMLURL) == "" {
		return card.Card{}, ErrMissingURL
	}
	labels := item.Topics
	if labels == nil {
		labels = []string{}
	}
	return card.Card{
		UID:         card.TextUID(item.FullName),
		Name:        item.Name,
		Path:        pathFromFullName(item.FullName),
		URL:         strings.TrimSpace(*item.HTMLURL),
		Description: desc,
		Labels:      append([]string(nil), labels...),
		Properties:  card.EmptyProperties(),
	}, nil
}

func pathFromFullName(fullName string) string {
	if fullName == "" {
		return ""
	}
	idx := strings.LastIndex(fullName, "/")
	if idx <= 0 {
		return ""
	}
	return fullName[:idx]
}
