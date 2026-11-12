// Маппинг элемента GitLab list projects в card.Card (только list, без enrichment).
package gitlab

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// ErrMissingURL — в элементе list нет непустого URL страницы репозитория.
var ErrMissingURL = errors.New("repository list item is missing url")

type projectItem struct {
	ID                json.Number `json:"id"`
	Name              string      `json:"name"`
	PathWithNamespace string      `json:"path_with_namespace"`
	Description       *string     `json:"description"`
	WebURL            *string     `json:"web_url"`
	Topics            []string    `json:"topics"`
}

// MapProject строит карточку из элемента JSON-массива GET /api/v4/projects.
func MapProject(raw json.RawMessage) (card.Card, error) {
	var p projectItem
	if err := json.Unmarshal(raw, &p); err != nil {
		return card.Card{}, err
	}
	desc := ""
	if p.Description != nil {
		desc = *p.Description
	}
	if p.ID == "" {
		return card.Card{}, errors.New("gitlab project is missing id")
	}
	id, err := p.ID.Int64()
	if err != nil {
		return card.Card{}, err
	}
	if p.WebURL == nil || strings.TrimSpace(*p.WebURL) == "" {
		return card.Card{}, ErrMissingURL
	}
	labels := p.Topics
	if labels == nil {
		labels = []string{}
	}
	return card.Card{
		UID:         card.NumberUID(id),
		Name:        p.Name,
		Path:        pathFromNamespace(p.PathWithNamespace),
		URL:         strings.TrimSpace(*p.WebURL),
		Description: desc,
		Labels:      append([]string(nil), labels...),
		Properties:  card.EmptyProperties(),
	}, nil
}

func pathFromNamespace(pathWithNS string) string {
	for i := len(pathWithNS) - 1; i >= 0; i-- {
		if pathWithNS[i] == '/' {
			if i == 0 {
				return ""
			}
			return pathWithNS[:i]
		}
	}
	return ""
}
