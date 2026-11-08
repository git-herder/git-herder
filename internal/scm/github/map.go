// Маппинг элемента GitHub Search API в card.Card (только list, без enrichment).
package github

import (
	"encoding/json"
	"strings"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

type searchItem struct {
	FullName    string   `json:"full_name"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
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
	labels := item.Topics
	if labels == nil {
		labels = []string{}
	}
	return card.Card{
		UID:         item.FullName,
		Name:        item.Name,
		Path:        pathFromFullName(item.FullName),
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
