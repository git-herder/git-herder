// Пакет scm: фабрика SCM-клиентов и контракт перечисления репозиториев.
package scm

import (
	"context"
	"time"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

// DefaultPerPage — размер страницы list, пока per_page не зафиксирован в спеке профиля.
const DefaultPerPage = 100

// NameValue — пара key/value для server-side равенства (--qa с оператором =).
type NameValue struct {
	Name  string
	Value string
}

// ConnConfig — подключение к SCM для list-запросов.
type ConnConfig struct {
	Upstream        string
	Host            string
	Token           string
	PerPage         int
	Scope           string // affiliated | all
	IncludeInactive bool
}

// ListingCriteria — критерии, уже разобранные из argv/профиля.
type ListingCriteria struct {
	Paths []string
	Names []string
	Tags  []string // только наличие тега (qt без =null)
	Attrs []NameValue
}

// Listing — перечисление репозиториев одним list-запросом (страницы).
type Listing interface {
	PagesCount(ctx context.Context) (int, error)
	Repositories(ctx context.Context, page int) ([]card.Card, error)
	RateLimit() *int
	RateLimitReset() *time.Time
	RateLimitUsed() *int
}

// ListingBuilder — fluent-конфигурация listing перед Build.
type ListingBuilder interface {
	SetName(string) ListingBuilder
	SetPath(string) ListingBuilder
	SetTag(string) ListingBuilder
	SetAttribute(name, value string) ListingBuilder
	Build() (Listing, error)
}
