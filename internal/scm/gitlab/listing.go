// GitlabScmListing: билдер и list через GET /api/v4/projects.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// NewListingBuilder создаёт билдер GitLab listing.
func NewListingBuilder(transport *scmhttp.Transport, cfg conn.Config) scm.ListingBuilder {
	perPage := cfg.PerPage
	if perPage <= 0 {
		perPage = scm.DefaultPerPage
	}
	return &listingBuilder{
		transport: transport,
		cfg:       cfg,
		perPage:   perPage,
	}
}

type listingBuilder struct {
	transport *scmhttp.Transport
	cfg       conn.Config
	perPage   int
	names     []string
	paths     []string
	tags      []string
	attrs     []scm.NameValue
}

func (b *listingBuilder) SetName(name string) scm.ListingBuilder {
	b.names = append(b.names, name)
	return b
}

func (b *listingBuilder) SetPath(path string) scm.ListingBuilder {
	b.paths = append(b.paths, path)
	return b
}

func (b *listingBuilder) SetTag(name string) scm.ListingBuilder {
	b.tags = append(b.tags, name)
	return b
}

func (b *listingBuilder) SetAttribute(name, value string) scm.ListingBuilder {
	b.attrs = append(b.attrs, scm.NameValue{Name: name, Value: value})
	return b
}

func (b *listingBuilder) Build() (scm.Listing, error) {
	return &listing{
		transport: b.transport,
		baseURL:   b.cfg.Host,
		perPage:   b.perPage,
		cfg:       b.cfg,
		tags:      append([]string(nil), b.tags...),
		attrs:     append([]scm.NameValue(nil), b.attrs...),
	}, nil
}

type listing struct {
	transport *scmhttp.Transport
	baseURL   string
	perPage   int
	cfg       conn.Config
	tags      []string
	attrs     []scm.NameValue

	pagesReady bool
	totalPages int
}

func (l *listing) PagesCount(ctx context.Context) (int, error) {
	if err := l.ensurePages(ctx); err != nil {
		return 0, err
	}
	return l.totalPages, nil
}

func (l *listing) Repositories(ctx context.Context, page int) ([]card.Card, error) {
	if page < 1 {
		return nil, fmt.Errorf("page %d out of range", page)
	}
	if err := l.ensurePages(ctx); err != nil {
		return nil, err
	}
	if page > l.totalPages {
		return nil, fmt.Errorf("page %d out of range", page)
	}
	resp, err := l.fetchPage(ctx, page)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("gitlab list returned HTTP %d", resp.Status)
	}
	return decodeProjects(resp.Body)
}

func (l *listing) RateLimit() *int { return nil }

func (l *listing) RateLimitReset() *time.Time { return nil }

func (l *listing) RateLimitUsed() *int { return nil }

func (l *listing) ensurePages(ctx context.Context) error {
	if l.pagesReady {
		return nil
	}
	resp, err := l.fetchPage(ctx, 1)
	if err != nil {
		return err
	}
	pages := parseTotalPages(resp.Header.Get("X-Total-Pages"))
	if pages <= 0 {
		pages = 1
	}
	l.totalPages = pages
	l.pagesReady = true
	return nil
}

func parseTotalPages(h string) int {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	n, err := strconv.Atoi(h)
	if err != nil {
		return 0
	}
	return n
}

func (l *listing) listQuery(page int) url.Values {
	qv := url.Values{}
	qv.Set("page", strconv.Itoa(page))
	qv.Set("per_page", strconv.Itoa(l.perPage))
	if l.cfg.SelectionScope == resolved.SelectionScopeAffiliated {
		qv.Set("membership", "true")
	}
	if !l.cfg.IncludeInactive {
		qv.Set("active", "true")
	}
	if len(l.tags) > 0 {
		qv.Set("topic", strings.Join(l.tags, ","))
	}
	for _, a := range l.attrs {
		qv.Set("custom_attributes["+a.Name+"]", a.Value)
	}
	return qv
}

func (l *listing) fetchPage(ctx context.Context, page int) (scmhttp.Response, error) {
	return l.transport.Get(ctx, l.baseURL, "/projects", l.listQuery(page))
}

func decodeProjects(body []byte) ([]card.Card, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	if items == nil {
		return []card.Card{}, nil
	}
	out := make([]card.Card, 0, len(items))
	for _, raw := range items {
		c, err := MapProject(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
