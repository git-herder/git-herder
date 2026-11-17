// Пакет cli: коды завершения для ошибок CLI.
package cli

import "github.com/git-herder/git-herder/internal/appexit"

// ErrExitCode оборачивает ошибку с кодом выхода.
func ErrExitCode(code int, err error) error {
	return appexit.WithCode(code, err)
}
