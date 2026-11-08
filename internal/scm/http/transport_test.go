package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host, upstream, want string
	}{
		{"github.com", "github", "https://api.github.com"},
		{"ghe.example", "github", "https://ghe.example/api"},
		{"gitlab.com", "gitlab", "https://gitlab.com/api/v4"},
	}
	for _, tc := range cases {
		got, err := BaseURL(tc.host, tc.upstream)
		if err != nil || got != tc.want {
			t.Fatalf("%s/%s: got %q err %v", tc.host, tc.upstream, got, err)
		}
	}
}

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

func TestBaseURLErrors(t *testing.T) {
	t.Parallel()
	if _, err := BaseURL("", "github"); err == nil {
		t.Fatal("empty host")
	}
	if _, err := BaseURL("h", "svn"); err == nil {
		t.Fatal("unknown upstream")
	}
}
