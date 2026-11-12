package http

import (
	"net/http"
	"strconv"
	"time"
)

// Quota — снимок квоты платформы после разбора ответа.
type Quota struct {
	Limit     *int
	Remaining *int
	Used      *int
	Reset     time.Time
	Set       bool
}

// LimitPolicy — поведение при нулевом остатке.
type LimitPolicy int

const (
	// LimitAwait удерживает вызов до момента сброса.
	LimitAwait LimitPolicy = iota
	// LimitAbort не выполняет запрос.
	LimitAbort
)

// ParseGitHubQuota читает заголовки GitHub. Момент сброса — epoch + 1 с.
func ParseGitHubQuota(h http.Header) Quota {
	q := Quota{}
	if h == nil {
		return q
	}
	if n, err := strconv.Atoi(h.Get("X-Ratelimit-Limit")); err == nil {
		q.Limit = &n
		q.Set = true
	}
	if n, err := strconv.Atoi(h.Get("X-Ratelimit-Remaining")); err == nil {
		q.Remaining = &n
		q.Set = true
	}
	if n, err := strconv.Atoi(h.Get("X-Ratelimit-Used")); err == nil {
		q.Used = &n
		q.Set = true
	}
	if sec, err := strconv.ParseInt(h.Get("X-Ratelimit-Reset"), 10, 64); err == nil {
		q.Reset = time.Unix(sec, 0).UTC().Add(time.Second)
		q.Set = true
	}
	return q
}

// ParseGitLabQuota читает заголовки GitLab. Момент сброса — epoch как в заголовке.
func ParseGitLabQuota(h http.Header) Quota {
	q := Quota{}
	if h == nil {
		return q
	}
	if n, err := strconv.Atoi(h.Get("Ratelimit-Limit")); err == nil {
		q.Limit = &n
		q.Set = true
	}
	if n, err := strconv.Atoi(h.Get("Ratelimit-Remaining")); err == nil {
		q.Remaining = &n
		q.Set = true
	}
	if n, err := strconv.Atoi(h.Get("Ratelimit-Observed")); err == nil {
		q.Used = &n
		q.Set = true
	}
	if sec, err := strconv.ParseInt(h.Get("Ratelimit-Reset"), 10, 64); err == nil {
		q.Reset = time.Unix(sec, 0).UTC()
		q.Set = true
	}
	return q
}
