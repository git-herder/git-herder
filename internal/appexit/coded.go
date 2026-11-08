// Пакет appexit связывает ошибки с кодами завершения процесса.
package appexit

// CodedError — ошибка с кодом выхода CLI.
type CodedError struct {
	Code int
	Err  error
}

func (e CodedError) Error() string { return e.Err.Error() }
func (e CodedError) Unwrap() error { return e.Err }

// WithCode оборачивает err кодом выхода.
func WithCode(code int, err error) error {
	return CodedError{Code: code, Err: err}
}
