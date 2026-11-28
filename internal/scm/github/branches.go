package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
	scmhttp "github.com/git-herder/git-herder/internal/scm/http"
)

// BranchesEnricher точечно проверяет имена --has-branch через GET …/branches/{branch}.
type BranchesEnricher struct {
	transport *scmhttp.Transport
	baseURL   string
	names     []string
}

// NewBranchesEnricher создаёт задание обогащения branches.
func NewBranchesEnricher(transport *scmhttp.Transport, baseURL string, names []string) *BranchesEnricher {
	return &BranchesEnricher{
		transport: transport,
		baseURL:   baseURL,
		names:     append([]string(nil), names...),
	}
}

func (e *BranchesEnricher) ID() string { return "branches" }

func (e *BranchesEnricher) Apply(ctx context.Context, c *card.Card) (err error) {
	fullName := ""
	if c != nil {
		fullName = c.FullName()
	}
	leave := logging.Enter("ctx", ctx, "id", e.ID(), "fullName", fullName, "names", len(e.names))
	defer func() { leave("err", err) }()
	owner, repo, ok := strings.Cut(c.UID.String(), "/")
	if !ok || owner == "" || repo == "" {
		return errBrokenUID(c.UID.String())
	}
	confirmed := make([]string, 0, len(e.names))
	seen := map[string]struct{}{}
	asked := map[string]struct{}{}
	for _, name := range e.names {
		if _, ok := asked[name]; ok {
			continue
		}
		asked[name] = struct{}{}
		path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/branches/" + url.PathEscape(name)
		resp, err := e.transport.Get(ctx, e.baseURL, path, nil)
		if err != nil {
			return err
		}
		switch resp.Status {
		case 200:
			got, err := confirmedBranchName(resp.Body)
			if err != nil {
				return err
			}
			if _, dup := seen[got]; dup {
				continue
			}
			seen[got] = struct{}{}
			confirmed = append(confirmed, got)
		case 404:
		default:
			return &scm.HTTPError{Status: resp.Status, What: "github branch"}
		}
	}
	c.SetBranches(confirmed)
	return nil
}

func confirmedBranchName(body []byte) (string, error) {
	trim := bytes.TrimSpace(body)
	if len(trim) == 0 || trim[0] != '{' {
		return "", fmt.Errorf("branch response is not an object")
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(trim, &obj); err != nil {
		return "", err
	}
	if obj.Name == "" {
		return "", fmt.Errorf("branch response missing name")
	}
	return obj.Name, nil
}
