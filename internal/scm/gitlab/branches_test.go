package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/scm/card"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

func TestBranchesEnricher(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		hits = append(hits, r.URL.EscapedPath())
		switch {
		case strings.Contains(r.URL.EscapedPath(), "release%2F1.2"):
			_, _ = w.Write([]byte(`{"name":"release/1.2"}`))
		case strings.HasSuffix(r.URL.Path, "/repository/branches/missing"):
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/repository/branches/bad"):
			w.WriteHeader(http.StatusBadGateway)
		default:
			t.Errorf("path %s", r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "tok")
	c := card.Card{UID: card.NumberUID(7), Branches: []string{"old"}}
	job := NewBranchesEnricher(tr, srv.URL, []string{"release/1.2", "missing"})
	if job.ID() != "branches" {
		t.Fatal(job.ID())
	}
	if err := job.Apply(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Branches) != 1 || c.Branches[0] != "release/1.2" {
		t.Fatalf("%#v", c.Branches)
	}
	if len(hits) != 2 || !strings.Contains(hits[0], "/projects/7/repository/branches/release%2F1.2") {
		t.Fatalf("hits %#v", hits)
	}
	stale := card.Card{UID: card.NumberUID(7), Branches: []string{"old"}}
	if err := NewBranchesEnricher(tr, srv.URL, []string{"bad"}).Apply(context.Background(), &stale); err == nil {
		t.Fatal("expected http error")
	}
	if len(stale.Branches) != 1 || stale.Branches[0] != "old" {
		t.Fatalf("partial %#v", stale.Branches)
	}
	text := card.Card{UID: card.TextUID("nope")}
	if err := job.Apply(context.Background(), &text); err == nil {
		t.Fatal("text uid")
	}
}
