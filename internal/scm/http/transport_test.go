package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

func TestPatchJSON(t *testing.T) {
	t.Parallel()
	var method, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	tr := NewTransport(srv.Client(), "tok")
	resp, err := tr.Patch(context.Background(), srv.URL, "repos/acme/api/properties/values", []byte(`{"properties":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusNoContent || method != http.MethodPatch || gotBody != `{"properties":[]}` {
		t.Fatalf("status %d method %s body %s", resp.Status, method, gotBody)
	}
}

func TestPostJSONAndAbort(t *testing.T) {
	t.Parallel()
	var calls int
	var method, ctype, auth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		method = r.Method
		ctype = r.Header.Get("Content-Type")
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("X-Ratelimit-Reset", "100")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tr := NewTransport(srv.Client(), "tok")
	tr.SetQuota(ParseGitHubQuota, LimitAbort, nil)
	resp, err := tr.Post(context.Background(), srv.URL, "orgs/acme/repos", []byte(`{"name":"r"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusCreated || method != http.MethodPost || ctype != "application/json" || auth != "Bearer tok" || gotBody != `{"name":"r"}` {
		t.Fatalf("status %d method %s type %s auth %s body %s", resp.Status, method, ctype, auth, gotBody)
	}
	if _, err = tr.Post(context.Background(), srv.URL, "orgs/acme/repos", []byte(`{"name":"r"}`)); err != ErrRateLimit {
		t.Fatalf("abort %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
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
