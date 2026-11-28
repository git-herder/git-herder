package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/retrying"
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
		case strings.Contains(r.URL.EscapedPath(), "feature%2Ffoo"):
			_, _ = w.Write([]byte(`{"name":"feature/foo","protected":false}`))
		case strings.HasSuffix(r.URL.Path, "/branches/missing"):
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/branches/main"):
			_, _ = w.Write([]byte(`{"name":"main"}`))
		case strings.HasSuffix(r.URL.Path, "/branches/bad"):
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasSuffix(r.URL.Path, "/branches/empty"):
			_, _ = w.Write([]byte(`{"name":""}`))
		case strings.HasSuffix(r.URL.Path, "/branches/array"):
			_, _ = w.Write([]byte(`[{"name":"array"}]`))
		default:
			t.Errorf("path %s", r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "tok")
	job := NewBranchesEnricher(tr, srv.URL, []string{"feature/foo", "feature/foo", "missing", "main"})
	if job.ID() != "branches" {
		t.Fatal(job.ID())
	}
	c := card.Card{UID: card.TextUID("acme/demo"), Branches: []string{"stale"}}
	if err := job.Apply(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Branches) != 2 || c.Branches[0] != "feature/foo" || c.Branches[1] != "main" {
		t.Fatalf("%#v", c.Branches)
	}
	var encoded, listed int
	for _, hit := range hits {
		if strings.Contains(hit, "feature%2Ffoo") {
			encoded++
		}
		if strings.HasSuffix(hit, "/branches") {
			listed++
		}
	}
	if encoded != 1 || listed != 0 {
		t.Fatalf("hits %#v", hits)
	}

	stale := card.Card{UID: card.TextUID("acme/demo"), Branches: []string{"stale"}}
	if err := NewBranchesEnricher(tr, srv.URL, []string{"main", "bad"}).Apply(context.Background(), &stale); err == nil {
		t.Fatal("expected http error")
	}
	if len(stale.Branches) != 1 || stale.Branches[0] != "stale" {
		t.Fatalf("partial write %#v", stale.Branches)
	}
	if err := NewBranchesEnricher(tr, srv.URL, []string{"empty"}).Apply(context.Background(), &stale); err == nil {
		t.Fatal("empty name")
	}
	if err := NewBranchesEnricher(tr, srv.URL, []string{"array"}).Apply(context.Background(), &stale); err == nil {
		t.Fatal("array body")
	}
	broken := card.Card{UID: card.TextUID("noseparator")}
	err := NewBranchesEnricher(tr, srv.URL, []string{"main"}).Apply(context.Background(), &broken)
	var abort *retrying.AbortError
	if err == nil || errors.As(err, &abort) {
		t.Fatalf("broken uid must be a retried job error, %v", err)
	}
}
