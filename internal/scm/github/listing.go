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

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

var qpQnPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// matchAllSearchQ — q, когда server-side фрагментов нет (listing § сборка q).
const matchAllSearchQ = "stars:>=0"

var errInvalidQPQN = errors.New("github qp/qn value contains disallowed characters")

// NewListingBuilder создаёт билдер GitHub listing.
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
	leave := logging.Enter("paths", b.paths, "names", b.names, "tags", b.tags)
	defer func() { leave("err", err) }()
	q, err := buildSearchQ(b.paths, b.names, b.attrs)
	if err != nil {
		return nil, err
	}
	return &listing{
		transport: b.transport,
		baseURL:   b.cfg.Host,
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
		return matchAllSearchQ, nil
	}
	return strings.Join(frags, "+"), nil
}

type listing struct {
	transport *scmhttp.Transport
	baseURL   string
	perPage   int
	q         string
	search    url.Values

	pagesReady bool
	totalPages int
	cache      map[int][]card.Card

	limit, used *int
	reset       *time.Time
}

// NewSearchListing строит listing ветки --scm-search (пары уже проверены).
func NewSearchListing(transport *scmhttp.Transport, cfg conn.Config, pairs []scm.SearchPair) (out scm.Listing, err error) {
	leave := logging.Enter("host", cfg.Host, "pairs", len(pairs))
	defer func() { leave("err", err) }()
	perPage := cfg.PerPage
	if perPage <= 0 {
		perPage = scm.DefaultPerPage
	}
	qv := url.Values{}
	for _, p := range pairs {
		qv.Set(p.Key, p.Value)
	}
	if strings.TrimSpace(qv.Get("q")) == "" {
		return nil, fmt.Errorf("invalid --scm-search: missing q")
	}
	return &listing{
		transport: transport,
		baseURL:   cfg.Host,
		perPage:   perPage,
		search:    qv,
	}, nil
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
	if l.cache != nil {
		if cards, ok := l.cache[page]; ok {
			return cards, nil
		}
	}
	resp, err := l.fetchPage(ctx, page)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, &scm.HTTPError{Status: resp.Status, What: "github list"}
	}
	return decodeSearchItems(resp.Body)
}

func (l *listing) RateLimit() *int            { return l.limit }
func (l *listing) RateLimitReset() *time.Time { return l.reset }
func (l *listing) RateLimitUsed() *int        { return l.used }

func (l *listing) ensurePages(ctx context.Context) error {
	if l.pagesReady {
		return nil
	}
	resp, err := l.fetchPage(ctx, 1)
	if err != nil {
		return err
	}
	l.applyRateLimit(resp.Header)
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "github list"}
	}
	pages, err := pagesFromLink(resp.Header.Get("Link"))
	if err != nil {
		return err
	}
	if pages == 0 {
		next := linkURL(resp.Header.Get("Link"), "next")
		if next != "" {
			return l.walkNext(ctx, resp)
		}
		pages = 1
	}
	l.totalPages = pages
	l.pagesReady = true
	return nil
}

// walkNext обходит страницы по rel=next, когда rel=last нет, и кеширует карточки.
func (l *listing) walkNext(ctx context.Context, first scmhttp.Response) error {
	cards, err := decodeSearchItems(first.Body)
	if err != nil {
		return err
	}
	l.cache = map[int][]card.Card{1: cards}
	page := 1
	next := linkURL(first.Header.Get("Link"), "next")
	for next != "" && page < 10000 {
		page++
		resp, err := l.transport.GetURL(ctx, next)
		if err != nil {
			return err
		}
		l.applyRateLimit(resp.Header)
		if resp.Status != 200 {
			return &scm.HTTPError{Status: resp.Status, What: "github list"}
		}
		cards, err = decodeSearchItems(resp.Body)
		if err != nil {
			return err
		}
		l.cache[page] = cards
		next = linkURL(resp.Header.Get("Link"), "next")
	}
	l.totalPages = page
	l.pagesReady = true
	return nil
}

func (l *listing) fetchPage(ctx context.Context, page int) (scmhttp.Response, error) {
	qv := url.Values{}
	if l.search != nil {
		for k, vs := range l.search {
			for _, v := range vs {
				qv.Add(k, v)
			}
		}
	} else {
		qv.Set("q", l.q)
	}
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

func linkURL(link, rel string) string {
	if link == "" {
		return ""
	}
	needle := `rel="` + rel + `"`
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, needle) {
			continue
		}
		uStart := strings.Index(part, "<")
		uEnd := strings.Index(part, ">")
		if uStart < 0 || uEnd <= uStart {
			continue
		}
		return part[uStart+1 : uEnd]
	}
	return ""
}

func pagesFromLink(link string) (int, error) {
	raw := linkURL(link, "last")
	if raw == "" {
		return 0, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0, err
	}
	p := u.Query().Get("page")
	if p == "" {
		return 0, nil
	}
	return strconv.Atoi(p)
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
