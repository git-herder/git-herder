package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/conn"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

func TestPropertiesEnricher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/demo/properties/values" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"property_name":"gone","value":null},
			{"property_name":"ok","value":true},
			{"property_name":"n","value":1.0},
			{"property_name":"frac","value":1.5},
			{"property_name":"exp","value":1e2},
			{"property_name":"when","value":"2026-01-02T03:04:05.000+00:00"},
			{"property_name":"arr","value":[1,2]},
			{"property_name":"obj","value":{"a":1}}
		]`))
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "tok")
	job := NewPropertiesEnricher(tr, srv.URL)
	if job.ID() != "properties" {
		t.Fatal(job.ID())
	}
	c := card.Card{UID: card.TextUID("acme/demo"), Properties: card.EmptyProperties()}
	if err := job.Apply(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Properties["gone"]; ok {
		t.Fatal("null kept")
	}
	if c.Properties["ok"] != true || c.Properties["when"] == "" {
		t.Fatalf("%+v", c.Properties)
	}
	broken := card.Card{UID: card.TextUID("noseparator")}
	if err := job.Apply(context.Background(), &broken); err == nil {
		t.Fatal("expected broken uid")
	}
	_, err := NewSearchListing(tr, conn.Config{Host: srv.URL}, nil)
	if err == nil {
		t.Fatal("search requires q")
	}
}

func TestWalkNextPages(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"items":[{"full_name":"o/b","name":"b","html_url":"https://github.com/o/b"}]}`))
			return
		}
		w.Header().Set("Link", `<`+srv.URL+`/search/repositories?q=code&page=2>; rel="next"`)
		_, _ = w.Write([]byte(`{"items":[{"full_name":"o/a","name":"a","html_url":"https://github.com/o/a"}]}`))
	}))
	defer srv.Close()
	tr := scmhttp.NewTransport(srv.Client(), "")
	b := NewListingBuilder(tr, conn.Config{Host: srv.URL})
	list, err := b.SetName("code").Build()
	if err != nil {
		t.Fatal(err)
	}
	n, err := list.PagesCount(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("pages %d %v", n, err)
	}
	cards, err := list.Repositories(context.Background(), 2)
	if err != nil || len(cards) != 1 || cards[0].Name != "b" {
		t.Fatalf("%+v %v", cards, err)
	}
	if _, err = MapSearchItem([]byte(`{"full_name":"a","name":"b"}`)); err == nil {
		t.Fatal("missing url")
	}
}
