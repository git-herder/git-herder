package scm

import "fmt"

// HTTPError — ответ SCM с кодом, отличным от успеха.
type HTTPError struct {
	Status int
	What   string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "HTTP error"
	}
	return fmt.Sprintf("%s returned HTTP %d", e.What, e.Status)
}
