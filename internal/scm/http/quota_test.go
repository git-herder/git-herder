package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuotaAbortAndAwait(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-Ratelimit-Limit", "10")
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("X-Ratelimit-Used", "10")
		w.Header().Set("X-Ratelimit-Reset", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	abort := NewTransport(srv.Client(), "t")
	abort.SetQuota(ParseGitHubQuota, LimitAbort, nil)
	if _, err := abort.Get(context.Background(), srv.URL, "a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := abort.Get(context.Background(), srv.URL, "b", nil); err != ErrRateLimit {
		t.Fatalf("abort %v", err)
	}

	waited := false
	await := NewTransport(srv.Client(), "")
	await.SetQuota(ParseGitHubQuota, LimitAwait, func(time.Time) { waited = true })
	await.SetClock(func() time.Time { return time.Unix(50, 0) }, func(time.Duration) {})
	if _, err := await.Get(context.Background(), srv.URL, "c", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := await.GetURL(context.Background(), srv.URL+"/d"); err != nil {
		t.Fatal(err)
	}
	if !waited {
		t.Fatal("expected wait callback")
	}
	if ParseGitHubQuota(nil).Set {
		t.Fatal("nil header")
	}
	if calls < 3 {
		t.Fatalf("calls %d", calls)
	}
}
