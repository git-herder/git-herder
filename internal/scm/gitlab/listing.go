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

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/conn"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

// apiV4 — префикс REST относительно base URL инстанса (host), не часть host.
const apiV4 = "/api/v4"

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

func (b *listingBuilder) Build() (out scm.Listing, err error) {
	leave := logging.Enter("tags", b.tags, "attrs", len(b.attrs))
	defer func() { leave("err", err) }()
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

	limit, used *int
	reset       *time.Time
}

func (l *listing) PagesCount(ctx context.Context) (n int, err error) {
	leave := logging.Enter("ctx", ctx)
	defer func() { leave("pages", n, "err", err) }()
	if err := l.ensurePages(ctx); err != nil {
		return 0, err
	}
	return l.totalPages, nil
}

func (l *listing) Repositories(ctx context.Context, page int) (cards []card.Card, err error) {
	leave := logging.Enter("ctx", ctx, "page", page)
	defer func() { leave("cards", cards, "err", err) }()
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
		return nil, &scm.HTTPError{Status: resp.Status, What: "gitlab list"}
	}
	return decodeProjects(resp.Body)
}

func (l *listing) RateLimit() *int { return l.limit }

func (l *listing) RateLimitReset() *time.Time { return l.reset }

func (l *listing) RateLimitUsed() *int { return l.used }

func (l *listing) rememberQuota(h map[string][]string) {
	q := scmhttp.ParseGitLabQuota(h)
	if q.Limit != nil {
		l.limit = q.Limit
	}
	if q.Used != nil {
		l.used = q.Used
	}
	if !q.Reset.IsZero() {
		t := q.Reset
		l.reset = &t
	}
}

func (l *listing) ensurePages(ctx context.Context) error {
	if l.pagesReady {
		return nil
	}
	resp, err := l.fetchPage(ctx, 1)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "gitlab list"}
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
	resp, err := l.transport.Get(ctx, l.baseURL, apiV4+"/projects", l.listQuery(page))
	if err != nil {
		return resp, err
	}
	l.rememberQuota(resp.Header)
	return resp, nil
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
