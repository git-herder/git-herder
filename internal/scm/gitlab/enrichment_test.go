package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

func TestEnrichersAndSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects/5/labels":
			_, _ = w.Write([]byte(`[{"name":"a"},{"name":"a"},{"name":""}]`))
		case r.URL.Path == "/projects/5/custom_attributes":
			_, _ = w.Write([]byte(`[{"key":"on","value":"TRUE"},{"key":"when","value":"2026-01-02T03:04:05.000+00:00"},{"key":"s","value":"plain"}]`))
		case r.URL.Path == "/search":
			w.Header().Set("X-Total-Pages", "1")
			_, _ = w.Write([]byte(`[{"id":5,"name":"n","path_with_namespace":"g/n","web_url":"https://gitlab.example/g/n"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "t")
	c := card.Card{UID: card.NumberUID(5), Labels: []string{"old"}, Properties: card.EmptyProperties()}
	labels := NewLabelsEnricher(tr, srv.URL)
	if labels.ID() != "labels" {
		t.Fatal(labels.ID())
	}
	if err := labels.Apply(context.Background(), &c); err != nil || len(c.Labels) != 1 || c.Labels[0] != "a" {
		t.Fatalf("%+v %v", c.Labels, err)
	}
	props := NewPropertiesEnricher(tr, srv.URL)
	if err := props.Apply(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	if c.Properties["on"] != true || c.Properties["s"] != "plain" {
		t.Fatalf("%+v", c.Properties)
	}
	bad := card.Card{UID: card.TextUID("x")}
	if err := labels.Apply(context.Background(), &bad); err == nil {
		t.Fatal("text uid")
	}
	if _, err := NewSearchListing(tr, conn.Config{Host: srv.URL}, []scm.SearchPair{{Key: "scope", Value: "issues"}}); err == nil {
		t.Fatal("bad scope")
	}
	list, err := NewSearchListing(tr, conn.Config{Host: srv.URL}, []scm.SearchPair{{Key: "search", Value: "n"}})
	if err != nil {
		t.Fatal(err)
	}
	n, err := list.PagesCount(context.Background())
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	cards, err := list.Repositories(context.Background(), 1)
	if err != nil || len(cards) != 1 {
		t.Fatal(err, cards)
	}
	if _, err = list.Repositories(context.Background(), 3); err == nil {
		t.Fatal("range")
	}
	if _, err = MapProject([]byte(`{"id":1,"name":"n"}`)); err == nil {
		t.Fatal("missing url")
	}
}
