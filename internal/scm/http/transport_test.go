package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBearer(t *testing.T) {
	t.Parallel()
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	tr := NewTransport(srv.Client(), "tok")
	_, err := tr.Get(context.Background(), srv.URL, "search/repositories", nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" {
		t.Fatalf("auth %q", auth)
	}
	if NewTransport(srv.Client(), "").Client() == nil {
		t.Fatal("client nil")
	}
}
