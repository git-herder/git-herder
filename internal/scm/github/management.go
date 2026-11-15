// GithubRepositoryManagement: проверка репозитория и создание в организации
// или у текущего пользователя. Повтора запроса здесь нет.
package github

import (
	"bytes"
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

// PropertyState — исход чтения одного пользовательского атрибута.
type PropertyState int

const (
	// PropertyAbsent — элемента нет или value равен null.
	PropertyAbsent PropertyState = iota
	// PropertyString — value является JSON-строкой.
	PropertyString
	// PropertyNotString — value есть, но это не строка.
	PropertyNotString
)

// PropertyValue — результат ReadProperty, если вызов не завершился ошибкой.
type PropertyValue struct {
	State PropertyState
	Value string
}

// PropertyReader читает одно значение пользовательского атрибута репозитория.
type PropertyReader interface {
	ReadProperty(ctx context.Context, owner, repo, property string) (PropertyValue, error)
}

var _ PropertyReader = (*management)(nil)

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

// ReadProperty читает одно значение пользовательского атрибута.
// Повтора запроса здесь нет. Разбор карточки обогащения не используется.
func (m *management) ReadProperty(ctx context.Context, owner, repo, property string) (got PropertyValue, err error) {
	leave := logging.Enter("owner", owner, "repo", repo, "property", property)
	defer func() { leave("state", got.State, "err", err) }()
	if err = validateProperty(owner, repo, property); err != nil {
		return PropertyValue{}, err
	}
	path := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/properties/values"
	resp, err := m.transport.Get(ctx, m.baseURL, path, nil)
	if err != nil {
		return PropertyValue{}, err
	}
	if resp.Status != http.StatusOK {
		return PropertyValue{}, &scm.HTTPError{Status: resp.Status, What: "github properties"}
	}
	return propertyValueFromBody(resp.Body, property)
}

func validateProperty(owner, repo, property string) error {
	if err := validateGitHub(owner, repo); err != nil {
		return err
	}
	if property == "" {
		return fmt.Errorf("%w: github property name must be non-empty", scm.ErrContract)
	}
	return nil
}

func propertyValueFromBody(body []byte, name string) (PropertyValue, error) {
	var items []struct {
		Name  string          `json:"property_name"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return PropertyValue{}, fmt.Errorf("github properties: %w", err)
	}
	var raw json.RawMessage
	found := false
	for _, it := range items {
		if it.Name == name {
			raw = it.Value
			found = true
		}
	}
	if !found {
		return PropertyValue{State: PropertyAbsent}, nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return PropertyValue{State: PropertyAbsent}, nil
	}
	if raw[0] != '"' {
		return PropertyValue{State: PropertyNotString}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return PropertyValue{}, fmt.Errorf("github properties: %w", err)
	}
	return PropertyValue{State: PropertyString, Value: s}, nil
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
