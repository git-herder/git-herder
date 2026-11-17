// Пакет http: ScmHttpTransport — один HTTP-вызов к SCM REST API.
// GET и POST делят квоту, заголовок Bearer и журнал; повтора здесь нет.
package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/git-herder/git-herder/internal/diag/logging"
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

// Transport выполняет авторизованные запросы к SCM API.
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
	return t.do(ctx, http.MethodGet, rawURL, nil)
}

// Get выполняет GET baseURL+path с query-параметрами.
func (t *Transport) Get(ctx context.Context, baseURL, path string, query url.Values) (resp Response, err error) {
	leave := logging.Enter("ctx", ctx, "baseURL", baseURL, "path", path, "query", query)
	defer func() { leave("status", resp.Status, "bodyLen", len(resp.Body), "err", err) }()
	rawURL := joinURL(baseURL, path)
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	return t.do(ctx, http.MethodGet, rawURL, nil)
}

// Post выполняет POST baseURL+path. Непустое тело уходит как JSON.
func (t *Transport) Post(ctx context.Context, baseURL, path string, body []byte) (resp Response, err error) {
	leave := logging.Enter("ctx", ctx, "baseURL", baseURL, "path", path, "bodyLen", len(body))
	defer func() { leave("status", resp.Status, "bodyLen", len(resp.Body), "err", err) }()
	return t.do(ctx, http.MethodPost, joinURL(baseURL, path), body)
}

// Patch выполняет PATCH baseURL+path. Непустое тело уходит как JSON.
func (t *Transport) Patch(ctx context.Context, baseURL, path string, body []byte) (resp Response, err error) {
	leave := logging.Enter("ctx", ctx, "baseURL", baseURL, "path", path, "bodyLen", len(body))
	defer func() { leave("status", resp.Status, "bodyLen", len(resp.Body), "err", err) }()
	return t.do(ctx, http.MethodPatch, joinURL(baseURL, path), body)
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (t *Transport) do(ctx context.Context, method, rawURL string, body []byte) (Response, error) {
	if err := t.waitIfExhausted(); err != nil {
		return Response{}, err
	}
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	loggedBody := ""
	if len(body) > 0 {
		loggedBody = string(body)
	}
	logging.Debug("http request", "method", req.Method, "url", rawURL, "headers", req.Header, "bodyLen", len(body), "body", loggedBody)
	resp, err := t.client.Do(req)
	if err != nil {
		logging.DebugFull("http response", "url", rawURL, "err", err, "bodyLen", 0, "body", "")
		return Response{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.DebugFull("http response", "url", rawURL, "status", resp.StatusCode, "err", err, "bodyLen", 0, "body", "")
		return Response{}, err
	}
	logging.DebugFull("http response", "status", resp.StatusCode, "headers", resp.Header, "bodyLen", len(respBody), "body", respBody, "err", error(nil))
	out := Response{
		Status:     resp.StatusCode,
		Body:       respBody,
		Header:     resp.Header.Clone(),
		RequestURL: rawURL,
	}
	if t.parse != nil {
		t.quota = t.parse(out.Header)
	}
	return out, nil
}
