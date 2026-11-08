// GithubScmListing: билдер и list через GET /search/repositories.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

var qpQnPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	errEmptySearchQuery = errors.New("github search query has no server-side fragments")
	errInvalidQPQN      = errors.New("github qp/qn value contains disallowed characters")
)

// NewListingBuilder создаёт билдер GitHub listing.
func NewListingBuilder(transport *scmhttp.Transport, cfg scm.ConnConfig) scm.ListingBuilder {
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
	cfg       scm.ConnConfig
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
	q, err := buildSearchQ(b.paths, b.names, b.attrs)
	if err != nil {
		return nil, err
	}
	base, err := scmhttp.BaseURL(b.cfg.Host, "github")
	if err != nil {
		return nil, err
	}
	return &listing{
		transport: b.transport,
		baseURL:   base,
		perPage:   b.perPage,
		q:         q,
	}, nil
}

// buildSearchQ собирает строку q для GitHub Search (exported for tests).
func buildSearchQ(paths, names []string, attrs []scm.NameValue) (string, error) {
	var frags []string
	for _, p := range paths {
		if !qpQnPattern.MatchString(p) {
			return "", errInvalidQPQN
		}
		frags = append(frags, "org:"+p, "user:"+p)
	}
	for _, n := range names {
		if !qpQnPattern.MatchString(n) {
			return "", errInvalidQPQN
		}
		frags = append(frags, n)
	}
	for _, a := range attrs {
		frags = append(frags, "props."+a.Name+":"+a.Value)
	}
	if len(frags) == 0 {
		return "", errEmptySearchQuery
	}
	return strings.Join(frags, "+"), nil
}

type listing struct {
	transport *scmhttp.Transport
	baseURL   string
	perPage   int
	q         string

	pagesReady bool
	totalPages int

	limit, used *int
	reset       *time.Time
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
		return nil, fmt.Errorf("github list returned HTTP %d", resp.Status)
	}
	return decodeSearchItems(resp.Body)
}

func (l *listing) RateLimit() *int       { return l.limit }
func (l *listing) RateLimitReset() *time.Time { return l.reset }
func (l *listing) RateLimitUsed() *int   { return l.used }

func (l *listing) ensurePages(ctx context.Context) error {
	if l.pagesReady {
		return nil
	}
	resp, err := l.fetchPage(ctx, 1)
	if err != nil {
		return err
	}
	l.applyRateLimit(resp.Header)
	pages, err := pagesFromLink(resp.Header.Get("Link"))
	if err != nil {
		return err
	}
	if pages == 0 {
		pages = 1
		var payload struct {
			TotalCount int `json:"total_count"`
		}
		_ = json.Unmarshal(resp.Body, &payload)
		if resp.Status == 200 && payload.TotalCount == 0 {
			pages = 1
		}
		if resp.Status == 200 && payload.TotalCount > 0 && pages == 1 {
			// без rel=last остаётся одна страница до уточнения через next (v1)
		}
	}
	l.totalPages = pages
	l.pagesReady = true
	return nil
}

func (l *listing) fetchPage(ctx context.Context, page int) (scmhttp.Response, error) {
	qv := url.Values{}
	qv.Set("q", l.q)
	qv.Set("page", strconv.Itoa(page))
	qv.Set("per_page", strconv.Itoa(l.perPage))
	resp, err := l.transport.Get(ctx, l.baseURL, "/search/repositories", qv)
	if err != nil {
		return resp, err
	}
	l.applyRateLimit(resp.Header)
	return resp, nil
}

func (l *listing) applyRateLimit(h map[string][]string) {
	hdr := make(map[string]string, len(h))
	for k, vs := range h {
		if len(vs) > 0 {
			hdr[strings.ToLower(k)] = vs[0]
		}
	}
	if v := hdr["x-ratelimit-limit"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			l.limit = &n
		}
	}
	if v := hdr["x-ratelimit-used"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			l.used = &n
		}
	}
	if v := hdr["x-ratelimit-reset"]; v != "" {
		if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
			t := time.Unix(sec, 0).UTC()
			l.reset = &t
		}
	}
}

func pagesFromLink(link string) (int, error) {
	if link == "" {
		return 0, nil
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="last"`) {
			continue
		}
		uStart := strings.Index(part, "<")
		uEnd := strings.Index(part, ">")
		if uStart < 0 || uEnd <= uStart {
			continue
		}
		u, err := url.Parse(part[uStart+1 : uEnd])
		if err != nil {
			return 0, err
		}
		p := u.Query().Get("page")
		if p == "" {
			continue
		}
		return strconv.Atoi(p)
	}
	return 0, nil
}

func decodeSearchItems(body []byte) ([]card.Card, error) {
	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		return []card.Card{}, nil
	}
	out := make([]card.Card, 0, len(payload.Items))
	for _, raw := range payload.Items {
		c, err := MapSearchItem(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
