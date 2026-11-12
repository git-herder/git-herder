package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

func TestBuilderFluentAndBuildFailsOnTagOnly(t *testing.T) {
	t.Parallel()
	tr := scmhttp.NewTransport(http.DefaultClient, "")
	b2 := NewListingBuilder(tr, conn.Config{Host: "https://api.github.com"})
	list, err := b2.SetTag("only").Build()
	if err != nil {
		t.Fatal(err)
	}
	if list.(*listing).q != matchAllSearchQ {
		t.Fatalf("q=%q", list.(*listing).q)
	}
	b := NewListingBuilder(tr, conn.Config{Host: "https://api.github.com"})
	if _, err = b.SetPath("p").SetName("n").SetAttribute("a", "b").Build(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildSearchQ(t *testing.T) {
	t.Parallel()
	q, err := buildSearchQ([]string{"jopos"}, []string{"code2"}, []scm.NameValue{{Name: "foo", Value: "bar"}})
	if err != nil || !strings.Contains(q, "org:jopos") || !strings.Contains(q, "code2") || !strings.Contains(q, "props.foo:bar") || strings.Contains(q, "stars:") {
		t.Fatalf("q=%q err=%v", q, err)
	}
	q, err = buildSearchQ(nil, nil, nil)
	if err != nil || q != matchAllSearchQ {
		t.Fatalf("q=%q err=%v", q, err)
	}
	_, err = buildSearchQ([]string{"bad*"}, nil, nil)
	if err != errInvalidQPQN {
		t.Fatalf("want invalid qp, got %v", err)
	}
}

func TestPagesFromLink(t *testing.T) {
	t.Parallel()
	link := `<https://api.github.com/search/repositories?q=x&page=1>; rel="first", <https://api.github.com/search/repositories?q=x&page=3>; rel="last"`
	n, err := pagesFromLink(link)
	if err != nil || n != 3 {
		t.Fatalf("pages=%d err=%v", n, err)
	}
}

func TestMapSearchItem(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"full_name":"org/repo","name":"repo","description":"d","html_url":"https://github.com/org/repo","topics":["a"]}`)
	c, err := MapSearchItem(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.UID.String() != "org/repo" || c.URL != "https://github.com/org/repo" || c.Path != "org" || c.Name != "repo" || c.Labels[0] != "a" {
		t.Fatalf("%+v", c)
	}
}

func TestListingPagesAndRateLimit(t *testing.T) {
	t.Parallel()
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s/search/repositories?q=x&page=2>; rel="last"`, base))
		w.Header().Set("X-Ratelimit-Limit", "5000")
		w.Header().Set("X-Ratelimit-Used", "1")
		w.Header().Set("X-Ratelimit-Reset", "1700000000")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_count":1,"items":[{"full_name":"o/r","name":"r","html_url":"https://github.com/o/r","topics":[]}]}`))
	}))
	defer srv.Close()
	base = srv.URL

	tr := scmhttp.NewTransport(srv.Client(), "")
	b := NewListingBuilder(tr, conn.Config{Host: srv.URL, PerPage: 10})
	b = b.SetName("code2")
	listing, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	n, err := listing.PagesCount(ctx)
	if err != nil || n != 2 {
		t.Fatalf("pages=%d err=%v", n, err)
	}
	cards, err := listing.Repositories(ctx, 1)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards len=%d err=%v", len(cards), err)
	}
	if listing.RateLimit() == nil || *listing.RateLimit() != 5000 {
		t.Fatalf("rate limit %+v", listing.RateLimit())
	}
	if listing.RateLimitReset() == nil || listing.RateLimitReset().Unix() != 1700000000 {
		t.Fatalf("reset %v", listing.RateLimitReset())
	}
	if listing.RateLimitUsed() == nil || *listing.RateLimitUsed() != 1 {
		t.Fatalf("used %+v", listing.RateLimitUsed())
	}
	_, err = listing.Repositories(ctx, 5)
	if err == nil {
		t.Fatal("expected out of range")
	}
	_ = time.UTC
}

func TestMapSearchItemEmptyDesc(t *testing.T) {
	t.Parallel()
	c, err := MapSearchItem([]byte(`{"full_name":"a","name":"b","html_url":"https://github.com/a"}`))
	if err != nil || c.Description != "" || c.Path != "" {
		t.Fatalf("%+v", c)
	}
}
