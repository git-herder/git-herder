// GitlabRepositoryManagement: проверка проекта, создание недостающих групп
// цепочки path и создание проекта. Повтора запроса здесь нет.
package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/scm"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

var errMissingGroupID = errors.New("gitlab group response is missing id")

// management — GitlabRepositoryManagement.
type management struct {
	transport *scmhttp.Transport
	baseURL   string
}

// NewRepositoryManagement собирает клиент GitLab для path и name карточки.
func NewRepositoryManagement(transport *scmhttp.Transport, baseURL string) scm.RepositoryCreator {
	return &management{transport: transport, baseURL: baseURL}
}

func validateGitLab(path, name string) error {
	if name == "" || path == "" {
		return fmt.Errorf("%w: gitlab path and name must be non-empty", scm.ErrContract)
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			return fmt.Errorf("%w: gitlab path has an empty segment", scm.ErrContract)
		}
	}
	return nil
}

// Exists выполняет GET /api/v4/projects/:id. 200 — есть, 404 — нет.
func (m *management) Exists(ctx context.Context, path, name string) (ok bool, err error) {
	leave := logging.Enter("path", path, "name", name)
	defer func() { leave("exists", ok, "err", err) }()
	if err = validateGitLab(path, name); err != nil {
		return false, err
	}
	resp, err := m.transport.Get(ctx, m.baseURL, apiV4+"/projects/"+url.PathEscape(path+"/"+name), nil)
	if err != nil {
		return false, err
	}
	switch resp.Status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, &scm.HTTPError{Status: resp.Status, What: "gitlab project"}
	}
}

// Create создаёт недостающие группы path и проект. Наличие проекта не смотрит.
func (m *management) Create(ctx context.Context, path, name string) (err error) {
	leave := logging.Enter("path", path, "name", name)
	defer func() { leave("err", err) }()
	if err = validateGitLab(path, name); err != nil {
		return err
	}
	segs := strings.Split(path, "/")
	var parent int64
	var hasParent bool
	var namespaceID int64
	for i, seg := range segs {
		prefix := strings.Join(segs[:i+1], "/")
		id, found, err := m.lookupGroup(ctx, prefix)
		if err != nil {
			return err
		}
		if !found {
			id, err = m.createGroup(ctx, seg, parent, hasParent)
			if err != nil {
				return err
			}
		}
		parent = id
		hasParent = true
		namespaceID = id
	}
	body, err := json.Marshal(struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		NamespaceID int64  `json:"namespace_id"`
	}{Name: name, Path: name, NamespaceID: namespaceID})
	if err != nil {
		return err
	}
	resp, err := m.transport.Post(ctx, m.baseURL, apiV4+"/projects", body)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusCreated {
		return &scm.HTTPError{Status: resp.Status, What: "gitlab project create"}
	}
	return nil
}

func (m *management) lookupGroup(ctx context.Context, prefix string) (int64, bool, error) {
	resp, err := m.transport.Get(ctx, m.baseURL, apiV4+"/groups/"+url.PathEscape(prefix), nil)
	if err != nil {
		return 0, false, err
	}
	switch resp.Status {
	case http.StatusOK:
		id, err := parseGroupID(resp.Body)
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	case http.StatusNotFound:
		return 0, false, nil
	default:
		return 0, false, &scm.HTTPError{Status: resp.Status, What: "gitlab group"}
	}
}

func (m *management) createGroup(ctx context.Context, segment string, parent int64, hasParent bool) (int64, error) {
	payload := struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		ParentID *int64 `json:"parent_id,omitempty"`
	}{Name: segment, Path: segment}
	if hasParent {
		p := parent
		payload.ParentID = &p
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	resp, err := m.transport.Post(ctx, m.baseURL, apiV4+"/groups", body)
	if err != nil {
		return 0, err
	}
	if resp.Status != http.StatusCreated {
		return 0, &scm.HTTPError{Status: resp.Status, What: "gitlab group create"}
	}
	return parseGroupID(resp.Body)
}

func parseGroupID(body []byte) (int64, error) {
	var v struct {
		ID *int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return 0, fmt.Errorf("gitlab group: %w", err)
	}
	if v.ID == nil {
		return 0, errMissingGroupID
	}
	return *v.ID, nil
}
