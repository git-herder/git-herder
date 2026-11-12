package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
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

func TestGetDebugLogMasksTokenAndBody(t *testing.T) {
	const secret = "super-secret-token"
	body := strings.Repeat("z", 3000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "7")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	tr, err := logging.New(logging.Options{Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	logging.Bind(tr)
	defer func() {
		logging.Unbind(tr)
		_ = tr.Close()
	}()

	q := url.Values{}
	q.Set("q", "org:acme")
	resp, err := NewTransport(srv.Client(), secret).Get(context.Background(), srv.URL, "search/repositories", q)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	logged, err := os.ReadFile(tr.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(logged)
	for _, want := range []string{
		"enter Transport.Get",
		`"loggerName":"scm.http"`,
		"method=GET",
		"search/repositories",
		"q=org%3Aacme",
		"status=200",
		"X-Ratelimit-Remaining=7",
		strings.Repeat("z", 3000),
		"Authorization=***",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, secret) {
		t.Fatalf("token leaked: %s", s)
	}
}
