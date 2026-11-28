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

func TestRunHasBranchEarlyAndLate(t *testing.T) {
	var branchHits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/branches/") {
			branchHits = append(branchHits, r.URL.Path)
			if strings.Contains(r.URL.Path, "/projects/2/") {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"name":"main"}`))
			return
		}
		w.Header().Set("X-Total-Pages", "1")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"keep","path_with_namespace":"g/keep","web_url":"https://gitlab.example/g/keep"},
			{"id":2,"name":"drop","path_with_namespace":"g/drop","web_url":"https://gitlab.example/g/drop"},
			{"id":3,"name":"other","path_with_namespace":"g/other","web_url":"https://gitlab.example/g/other"}
		]`))
	}))
	defer srv.Close()
	f := factory.NewClientFactoryWithClient(srv.Client())
	cfg := conn.Config{Upstream: resolved.UpstreamGitLab, Host: srv.URL, Token: "t", SelectionScope: resolved.SelectionScopeAll, IncludeInactive: true}
	crit := scm.Criteria{Names: []string{"keep|drop"}, Branches: []string{"main"}}
	cards, failed, err := Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort, Meter: NopMeter{}})
	if err != nil || failed {
		t.Fatalf("cards=%d failed=%v err=%v", len(cards), failed, err)
	}
	if len(cards) != 1 || cards[0].Name != "keep" || len(cards[0].Branches) != 1 {
		t.Fatalf("%+v", cards)
	}
	if len(branchHits) != 2 {
		t.Fatalf("early filter should skip other, hits %#v", branchHits)
	}
	for _, hit := range branchHits {
		if strings.Contains(hit, "/projects/3/") {
			t.Fatalf("enriched dropped card %s", hit)
		}
	}
}

func TestRunHasBranchContinueAndAbort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/branches/") {
			if strings.Contains(r.URL.Path, "/o/bad/") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"name":"main"}`))
			return
		}
		if !strings.Contains(r.URL.RawQuery, "q=involves") || strings.Contains(r.URL.RawQuery, "main") {
			t.Errorf("q %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"items":[
			{"full_name":"o/good","name":"good","html_url":"https://github.com/o/good"},
			{"full_name":"o/bad","name":"bad","html_url":"https://github.com/o/bad"}
		]}`))
	}))
	defer srv.Close()
	f := factory.NewClientFactoryWithClient(srv.Client())
	cfg := conn.Config{Upstream: resolved.UpstreamGitHub, Host: srv.URL, Token: "t", OnLimit: "abort"}
	crit := scm.Criteria{Branches: []string{"main"}}
	cards, failed, err := Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorContinue, Meter: NopMeter{}})
	if err != nil || !failed || len(cards) != 1 || cards[0].Name != "good" {
		t.Fatalf("cards=%+v failed=%v err=%v", cards, failed, err)
	}
	cards, failed, err = Run(context.Background(), f, cfg, crit, Options{Retries: 0, OnError: resolved.OnErrorAbort, Meter: NopMeter{}})
	if err == nil || failed || cards != nil {
		t.Fatalf("abort cards=%v failed=%v err=%v", cards, failed, err)
	}
}
