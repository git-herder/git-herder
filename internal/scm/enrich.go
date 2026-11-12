package scm

import (
	"context"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// Enricher догружает одно поле карточки и при успехе полностью его заменяет.
type Enricher interface {
	ID() string
	Apply(ctx context.Context, c *card.Card) error
}
