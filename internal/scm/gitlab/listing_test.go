package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm/conn"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

func TestBuilderAccumulatesClientSideCriteria(t *testing.T) {
	t.Parallel()
	tr := scmhttp.NewTransport(http.DefaultClient, "")
	b := NewListingBuilder(tr, conn.Config{Host: "https://gitlab.com/api/v4"})
	b.SetName("n").SetPath("p").SetTag("t").SetAttribute("k", "v")
	if _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
}

func TestMapProject(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"id":42,"name":"spec","path_with_namespace":"g/h/spec","web_url":"https://gitlab.example/g/h/spec","topics":["t"]}`)
	c, err := MapProject(raw)
	if err != nil || c.UID.String() != "42" || c.URL == "" || c.Path != "g/h" {
		t.Fatalf("%+v err=%v", c, err)
	}
}

func TestListingQueryAndPages(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects" {
			t.Errorf("path %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("X-Total-Pages", "2")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"a","path_with_namespace":"g/a","web_url":"https://gitlab.example/g/a"}]`))
	}))
	defer srv.Close()

	tr := scmhttp.NewTransport(srv.Client(), "x")
	cfg := conn.Config{
		Host:           srv.URL,
		SelectionScope: resolved.SelectionScopeAffiliated,
		PerPage:        20,
	}
	b := NewListingBuilder(tr, cfg)
	b = b.SetTag("foo").SetTag("bar").SetAttribute("k", "v")
	list, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := list.PagesCount(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "membership=true") || !strings.Contains(gotQuery, "active=true") {
		t.Fatalf("query %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "topic=foo%2Cbar") && !strings.Contains(gotQuery, "topic=foo,bar") {
		t.Fatalf("topic in %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "custom_attributes") {
		t.Fatalf("attrs in %q", gotQuery)
	}
	if list.RateLimit() != nil {
		t.Fatal("gitlab rate limit should be nil")
	}
	if list.RateLimitReset() != nil || list.RateLimitUsed() != nil {
		t.Fatal("gitlab rate limits should be nil")
	}
	cards, err := list.Repositories(ctx, 1)
	if err != nil || len(cards) != 1 || cards[0].Name != "a" {
		t.Fatalf("repos %+v err=%v", cards, err)
	}
	_, err = list.Repositories(ctx, 99)
	if err == nil {
		t.Fatal("expected page out of range")
	}
}

func TestPagesCountHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "x")
	list, err := NewListingBuilder(tr, conn.Config{Host: srv.URL}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := list.PagesCount(context.Background()); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestListingRemembersRateLimit(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Ratelimit-Limit", "2000")
		w.Header().Set("Ratelimit-Observed", "10")
		w.Header().Set("Ratelimit-Remaining", "1990")
		w.Header().Set("Ratelimit-Reset", "1700000000")
		w.Header().Set("X-Total-Pages", "1")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "x")
	list, err := NewListingBuilder(tr, conn.Config{Host: srv.URL}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := list.PagesCount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if list.RateLimit() == nil || *list.RateLimit() != 2000 {
		t.Fatalf("limit %v", list.RateLimit())
	}
	if list.RateLimitUsed() == nil || *list.RateLimitUsed() != 10 {
		t.Fatalf("used %v", list.RateLimitUsed())
	}
	if list.RateLimitReset() == nil || list.RateLimitReset().Unix() != 1700000000 {
		t.Fatalf("reset %v", list.RateLimitReset())
	}
}

func TestParseTotalPages(t *testing.T) {
	t.Parallel()
	if parseTotalPages("3") != 3 || parseTotalPages("") != 0 || parseTotalPages("x") != 0 {
		t.Fatal("parseTotalPages")
	}
}

func TestListQueryScopeAllIncludeInactive(t *testing.T) {
	t.Parallel()
	l := &listing{
		perPage: 10,
		cfg: conn.Config{
			SelectionScope:  resolved.SelectionScopeAll,
			IncludeInactive: true,
		},
	}
	q := l.listQuery(1)
	if q.Get("membership") != "" || q.Get("active") != "" {
		t.Fatalf("%v", q)
	}
}
