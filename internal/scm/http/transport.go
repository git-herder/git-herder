// Пакет http: ScmHttpTransport — обёртка GET-запросов к SCM REST API.
package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// ErrRateLimit — политика abort при нулевом остатке квоты.
var ErrRateLimit = errors.New("SCM rate limit exhausted")

const userAgent = "githerd"

// Response — результат одного HTTP-запроса к SCM.
type Response struct {
	Status     int
	Body       []byte
	Header     http.Header
	RequestURL string
}

// Transport выполняет авторизованные GET-запросы к SCM API.
type Transport struct {
	client *http.Client
	token  string
	parse  func(http.Header) Quota
	policy LimitPolicy
	onWait func(time.Time)
	quota  Quota
	now    func() time.Time
	sleep  func(time.Duration)
}

// NewTransport создаёт transport с заданным клиентом и bearer-токеном.
func NewTransport(client *http.Client, token string) *Transport {
	if client == nil {
		client = http.DefaultClient
	}
	return &Transport{client: client, token: strings.TrimSpace(token)}
}

// Client возвращает HTTP-клиент transport (для клонирования с новым токеном).
func (t *Transport) Client() *http.Client {
	return t.client
}

// SetQuota включает разбор квоты и политику нулевого остатка.
func (t *Transport) SetQuota(parse func(http.Header) Quota, policy LimitPolicy, onWait func(time.Time)) {
	t.parse = parse
	t.policy = policy
	t.onWait = onWait
}

// SetClock подменяет часы (тесты).
func (t *Transport) SetClock(now func() time.Time, sleep func(time.Duration)) {
	t.now = now
	t.sleep = sleep
}

func (t *Transport) clock() (func() time.Time, func(time.Duration)) {
	now := t.now
	if now == nil {
		now = time.Now
	}
	sleep := t.sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	return now, sleep
}

func (t *Transport) waitIfExhausted() error {
	if t.parse == nil || !t.quota.Set || t.quota.Remaining == nil || *t.quota.Remaining > 0 {
		return nil
	}
	if t.policy == LimitAbort {
		logging.Debug("quota wait", "remaining", 0, "reset", t.quota.Reset, "policy", "abort", "sleep", time.Duration(0))
		return ErrRateLimit
	}
	now, sleep := t.clock()
	if t.quota.Reset.IsZero() {
		logging.Debug("quota wait", "remaining", 0, "reset", t.quota.Reset, "policy", "await", "sleep", time.Duration(0))
		return nil
	}
	d := t.quota.Reset.Sub(now())
	sleepFor := time.Duration(0)
	if d > 0 {
		sleepFor = d
	}
	logging.Debug("quota wait", "remaining", 0, "reset", t.quota.Reset, "policy", "await", "sleep", sleepFor)
	if d > 0 {
		if t.onWait != nil {
			t.onWait(t.quota.Reset)
		}
		sleep(d)
	}
	return nil
}

// GetURL выполняет GET по абсолютному URL (пагинация rel=next).
func (t *Transport) GetURL(ctx context.Context, rawURL string) (resp Response, err error) {
	leave := logging.Enter("ctx", ctx, "url", rawURL)
	defer func() { leave("status", resp.Status, "bodyLen", len(resp.Body), "err", err) }()
	return t.do(ctx, rawURL)
}

// Get выполняет GET baseURL+path с query-параметрами.
func (t *Transport) Get(ctx context.Context, baseURL, path string, query url.Values) (resp Response, err error) {
	leave := logging.Enter("ctx", ctx, "baseURL", baseURL, "path", path, "query", query)
	defer func() { leave("status", resp.Status, "bodyLen", len(resp.Body), "err", err) }()
	base := strings.TrimRight(baseURL, "/")
	p := strings.TrimLeft(path, "/")
	rawURL := base + "/" + p
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	return t.do(ctx, rawURL)
}

func (t *Transport) do(ctx context.Context, rawURL string) (Response, error) {
	if err := t.waitIfExhausted(); err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	logging.Debug("http request", "method", req.Method, "url", rawURL, "headers", req.Header, "bodyLen", 0, "body", "")
	resp, err := t.client.Do(req)
	if err != nil {
		logging.DebugFull("http response", "url", rawURL, "err", err, "bodyLen", 0, "body", "")
		return Response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.DebugFull("http response", "url", rawURL, "status", resp.StatusCode, "err", err, "bodyLen", 0, "body", "")
		return Response{}, err
	}
	logging.DebugFull("http response", "status", resp.StatusCode, "headers", resp.Header, "bodyLen", len(body), "body", body, "err", error(nil))
	out := Response{
		Status:     resp.StatusCode,
		Body:       body,
		Header:     resp.Header.Clone(),
		RequestURL: rawURL,
	}
	if t.parse != nil {
		t.quota = t.parse(out.Header)
	}
	return out, nil
}
