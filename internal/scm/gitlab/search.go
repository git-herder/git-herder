package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// NewSearchListing строит listing ветки --scm-search (GET /search).
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
	if scope := qv.Get("scope"); scope != "" && !strings.EqualFold(scope, "projects") {
		return nil, fmt.Errorf("invalid --scm-search scope %q", scope)
	}
	qv.Set("scope", "projects")
	return &searchListing{
		transport: transport,
		baseURL:   cfg.Host,
		perPage:   perPage,
		query:     qv,
	}, nil
}

type searchListing struct {
	transport  *scmhttp.Transport
	baseURL    string
	perPage    int
	query      url.Values
	pagesReady bool
	totalPages int
}

func (l *searchListing) PagesCount(ctx context.Context) (n int, err error) {
	leave := logging.Enter("ctx", ctx)
	defer func() { leave("pages", n, "err", err) }()
	if err := l.ensure(ctx); err != nil {
		return 0, err
	}
	return l.totalPages, nil
}

func (l *searchListing) Repositories(ctx context.Context, page int) (cards []card.Card, err error) {
	leave := logging.Enter("ctx", ctx, "page", page)
	defer func() { leave("cards", cards, "err", err) }()
	if page < 1 {
		return nil, fmt.Errorf("page %d out of range", page)
	}
	if err := l.ensure(ctx); err != nil {
		return nil, err
	}
	if page > l.totalPages {
		return nil, fmt.Errorf("page %d out of range", page)
	}
	resp, err := l.fetch(ctx, page)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, &scm.HTTPError{Status: resp.Status, What: "gitlab search"}
	}
	return decodeProjects(resp.Body)
}

func (l *searchListing) RateLimit() *int { return nil }

func (l *searchListing) RateLimitReset() *time.Time { return nil }

func (l *searchListing) RateLimitUsed() *int { return nil }

func (l *searchListing) ensure(ctx context.Context) error {
	if l.pagesReady {
		return nil
	}
	resp, err := l.fetch(ctx, 1)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "gitlab search"}
	}
	pages := parseTotalPages(resp.Header.Get("X-Total-Pages"))
	if pages <= 0 {
		pages = 1
	}
	l.totalPages = pages
	l.pagesReady = true
	return nil
}

func (l *searchListing) fetch(ctx context.Context, page int) (scmhttp.Response, error) {
	qv := url.Values{}
	for k, vs := range l.query {
		for _, v := range vs {
			qv.Add(k, v)
		}
	}
	qv.Set("page", strconv.Itoa(page))
	qv.Set("per_page", strconv.Itoa(l.perPage))
	return l.transport.Get(ctx, l.baseURL, apiV4+"/search", qv)
}
