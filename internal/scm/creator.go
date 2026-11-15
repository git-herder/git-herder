// Пакет scm: контракт создания репозитория в SCM.
// Exists только проверяет наличие. Create не вызывает Exists и не
// подменяет ошибку API успехом. Ветку «если нет — создать» делает вызывающий.
package scm

import (
	"context"
	"errors"
)

// ErrContract — path или name не удовлетворяют контракту платформы.
// Возвращается до HTTP.
var ErrContract = errors.New("scm repository path or name is invalid")

// RepositoryCreator — клиент проверки и создания репозитория (ScmRepositoryCreator).
type RepositoryCreator interface {
	Exists(ctx context.Context, path, name string) (bool, error)
	Create(ctx context.Context, path, name string) error
}
