// Пакет retrying: RetryingQueue — последовательные повторы объектов прогона.
package retrying

import (
	"errors"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// Policy — что делать после исчерпания повторов.
type Policy int

const (
	// PolicyContinue фиксирует неуспех и переходит к следующему объекту.
	PolicyContinue Policy = iota
	// PolicyAbort останавливает прогон.
	PolicyAbort
)

// AbortError прерывает прогон сразу, без повторов (фатальная ошибка до очереди).
type AbortError struct{ Err error }

func (e *AbortError) Error() string {
	if e == nil || e.Err == nil {
		return "aborted"
	}
	return e.Err.Error()
}

func (e *AbortError) Unwrap() error { return e.Err }

// Execute обрабатывает объекты по очереди.
// retries — сколько раз повторить после первой ошибки.
// fatal(err) == true после исчерпания повторов прерывает прогон даже при PolicyContinue.
// Возвращает число окончательно неуспешных объектов.
func Execute[T any](items []T, retries int, policy Policy, fatal func(error) bool, fn func(T) error) (failed int, err error) {
	policyName := "continue"
	if policy == PolicyAbort {
		policyName = "abort"
	}
	leave := logging.Enter("count", len(items), "retries", retries, "policy", policyName, "fatal", fatal, "fn", fn)
	defer func() { leave("failed", failed, "err", err) }()

	if retries < 0 {
		retries = 0
	}
	type job struct {
		item  T
		fails int
	}
	q := make([]job, len(items))
	for i, it := range items {
		q[i] = job{item: it}
	}
	for len(q) > 0 {
		j := q[0]
		q = q[1:]
		callErr := fn(j.item)
		if callErr == nil {
			continue
		}
		attempt := j.fails + 1
		var abort *AbortError
		if errors.As(callErr, &abort) {
			logging.Debug("retry", "attempt", attempt, "err", callErr, "decision", "abort")
			return failed, callErr
		}
		if j.fails < retries {
			logging.Debug("retry", "attempt", attempt, "err", callErr, "decision", "retry")
			j.fails++
			q = append(q, j)
			continue
		}
		if policy == PolicyAbort || (fatal != nil && fatal(callErr)) {
			logging.Debug("retry", "attempt", attempt, "err", callErr, "decision", "abort")
			return failed, callErr
		}
		logging.Debug("retry", "attempt", attempt, "err", callErr, "decision", "continue")
		failed++
	}
	return failed, nil
}
