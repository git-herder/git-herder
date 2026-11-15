// GithubRepositoryManagement: проверка репозитория и создание в организации
// или у текущего пользователя. Повтора запроса здесь нет.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/scm"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// ErrOwnerNotFound — ни организация, ни пользователь с таким login не найдены.
var ErrOwnerNotFound = errors.New("github owner does not exist")

// ErrLoginMismatch — login текущего токена не совпал с сегментом path.
var ErrLoginMismatch = errors.New("github authenticated login does not match path")

type ownerKind int

const (
	ownerOrg ownerKind = iota
	ownerUser
)

// management — GithubRepositoryManagement.
type management struct {
	transport *scmhttp.Transport
	baseURL   string
}

// NewRepositoryManagement собирает клиент GitHub для path и name карточки.
func NewRepositoryManagement(transport *scmhttp.Transport, baseURL string) scm.RepositoryCreator {
	return &management{transport: transport, baseURL: baseURL}
}

func validateGitHub(path, name string) error {
	if name == "" || path == "" || strings.Contains(path, "/") {
		return fmt.Errorf("%w: github path must be a single segment and name must be non-empty", scm.ErrContract)
	}
	return nil
}

// Exists выполняет GET /repos/{owner}/{repo}. 200 — есть, 404 — нет.
func (m *management) Exists(ctx context.Context, path, name string) (ok bool, err error) {
	leave := logging.Enter("path", path, "name", name)
	defer func() { leave("exists", ok, "err", err) }()
	if err = validateGitHub(path, name); err != nil {
		return false, err
	}
	resp, err := m.transport.Get(ctx, m.baseURL, "repos/"+url.PathEscape(path)+"/"+url.PathEscape(name), nil)
	if err != nil {
		return false, err
	}
	switch resp.Status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, &scm.HTTPError{Status: resp.Status, What: "github repository"}
	}
}

// Create проверяет владельца и создаёт репозиторий. Наличие репозитория не смотрит.
func (m *management) Create(ctx context.Context, path, name string) (err error) {
	leave := logging.Enter("path", path, "name", name)
	defer func() { leave("err", err) }()
	if err = validateGitHub(path, name); err != nil {
		return err
	}
	kind, err := m.ownerKind(ctx, path)
	if err != nil {
		return err
	}
	postPath := "user/repos"
	if kind == ownerOrg {
		postPath = "orgs/" + url.PathEscape(path) + "/repos"
	} else if err = m.loginMatches(ctx, path); err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: name})
	if err != nil {
		return err
	}
	resp, err := m.transport.Post(ctx, m.baseURL, postPath, body)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusCreated {
		return &scm.HTTPError{Status: resp.Status, What: "github repository create"}
	}
	return nil
}

func (m *management) ownerKind(ctx context.Context, owner string) (ownerKind, error) {
	resp, err := m.transport.Get(ctx, m.baseURL, "orgs/"+url.PathEscape(owner), nil)
	if err != nil {
		return 0, err
	}
	switch resp.Status {
	case http.StatusOK:
		return ownerOrg, nil
	case http.StatusNotFound:
	default:
		return 0, &scm.HTTPError{Status: resp.Status, What: "github organization"}
	}
	resp, err = m.transport.Get(ctx, m.baseURL, "users/"+url.PathEscape(owner), nil)
	if err != nil {
		return 0, err
	}
	switch resp.Status {
	case http.StatusOK:
		return ownerUser, nil
	case http.StatusNotFound:
		return 0, ErrOwnerNotFound
	default:
		return 0, &scm.HTTPError{Status: resp.Status, What: "github user"}
	}
}

func (m *management) loginMatches(ctx context.Context, owner string) error {
	resp, err := m.transport.Get(ctx, m.baseURL, "user", nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return &scm.HTTPError{Status: resp.Status, What: "github authenticated user"}
	}
	var body struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return fmt.Errorf("github user: %w", err)
	}
	if body.Login != owner {
		return ErrLoginMismatch
	}
	return nil
}
