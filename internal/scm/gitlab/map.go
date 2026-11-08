// Маппинг элемента GitLab list projects в card.Card (только list, без enrichment).
package gitlab

import (
	"encoding/json"
	"strconv"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

type projectItem struct {
	ID                float64  `json:"id"`
	Name              string   `json:"name"`
	PathWithNamespace string   `json:"path_with_namespace"`
	Description       *string  `json:"description"`
	Topics            []string `json:"topics"`
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
	labels := p.Topics
	if labels == nil {
		labels = []string{}
	}
	return card.Card{
		UID:         strconv.FormatInt(int64(p.ID), 10),
		Name:        p.Name,
		Path:        pathFromNamespace(p.PathWithNamespace),
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
