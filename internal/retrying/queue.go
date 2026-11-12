// Пакет retrying: RetryingQueue — последовательные повторы объектов прогона.
package retrying

import "errors"

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
func Execute[T any](items []T, retries int, policy Policy, fatal func(error) bool, fn func(T) error) (int, error) {
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
	failed := 0
	for len(q) > 0 {
		j := q[0]
		q = q[1:]
		err := fn(j.item)
		if err == nil {
			continue
		}
		var abort *AbortError
		if errors.As(err, &abort) {
			return failed, err
		}
		if j.fails < retries {
			j.fails++
			q = append(q, j)
			continue
		}
		if policy == PolicyAbort || (fatal != nil && fatal(err)) {
			return failed, err
		}
		failed++
	}
	return failed, nil
}
