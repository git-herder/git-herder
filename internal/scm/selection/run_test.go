package selection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/conn"
	"github.com/git-herder/git-herder/internal/scm/factory"
)

func TestRunGitLabEnrichAndFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/labels"):
			_, _ = w.Write([]byte(`[{"name":"keep"},{"name":"keep"},{"name":"secret"}]`))
		case strings.Contains(r.URL.Path, "/custom_attributes"):
			_, _ = w.Write([]byte(`[{"key":"tier","value":"gold"},{"key":"flag","value":"true"},{"key":"n","value":"3"}]`))
		default:
			w.Header().Set("X-Total-Pages", "1")
			_, _ = w.Write([]byte(`[{"id":7,"name":"api","path_with_namespace":"g/api","web_url":"https://gitlab.example/g/api","topics":["old"]}]`))
		}
	}))
	defer srv.Close()
	f := factory.NewClientFactoryWithClient(srv.Client())
	cfg := conn.Config{Upstream: resolved.UpstreamGitLab, Host: srv.URL, Token: "t", SelectionScope: resolved.SelectionScopeAffiliated}
	crit := scm.Criteria{
		Names: []string{"api"},
		Tags:  []scm.TagFilter{{Name: "secret", Absent: true}},
		Attrs: []scm.AttrFilter{{Key: "tier", Op: ">=", Value: "gold"}},
	}
	cards, err := Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort, Meter: NopMeter{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Fatalf("secret label should exclude, %+v", cards)
	}
	crit.Tags = []scm.TagFilter{{Name: "keep"}}
	cards, err = Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort})
	if err != nil || len(cards) != 1 || cards[0].Labels[0] != "keep" {
		t.Fatalf("%+v %v", cards, err)
	}
	if cards[0].Properties["flag"] != true {
		t.Fatalf("props %+v", cards[0].Properties)
	}
}

func TestRunContinueAndForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	f := factory.NewClientFactoryWithClient(srv.Client())
	cfg := conn.Config{Upstream: resolved.UpstreamGitLab, Host: srv.URL, Token: "t"}
	crit := scm.Criteria{Names: []string{"a"}}
	cards, err := Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorContinue, Meter: NopMeter{}})
	if err != nil || len(cards) != 0 {
		t.Fatalf("continue cards=%d err=%v", len(cards), err)
	}
	cards, err = Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort, Meter: NopMeter{}})
	if err == nil || cards != nil {
		t.Fatal("expected abort")
	}

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer forbidden.Close()
	f = factory.NewClientFactoryWithClient(forbidden.Client())
	cfg.Host = forbidden.URL
	cfg.Upstream = resolved.UpstreamGitHub
	_, err = Run(context.Background(), f, cfg, scm.Criteria{Names: []string{"code"}}, Options{Retries: 0, OnError: resolved.OnErrorContinue})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("%v", err)
	}
}

func TestRunGitHubSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "q=org") {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"items":[{"full_name":"o/r","name":"r","html_url":"https://github.com/o/r","topics":[]}]}`))
	}))
	defer srv.Close()
	f := factory.NewClientFactoryWithClient(srv.Client())
	cfg := conn.Config{Upstream: resolved.UpstreamGitHub, Host: srv.URL, Token: "t", OnLimit: "abort"}
	crit := scm.Criteria{Search: true, SearchPairs: []scm.SearchPair{{Key: "q", Value: "org"}}}
	cards, err := Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort, Meter: &PtermMeter{}})
	if err != nil || len(cards) == 0 {
		t.Fatalf("cards=%d err=%v", len(cards), err)
	}
}
