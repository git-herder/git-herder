package github

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/Snowbridge/git-herder/internal/retrying"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// PropertiesEnricher догружает properties через GET /repos/{owner}/{repo}/properties/values.
type PropertiesEnricher struct {
	transport *scmhttp.Transport
	baseURL   string
}

// NewPropertiesEnricher создаёт задание обогащения properties.
func NewPropertiesEnricher(transport *scmhttp.Transport, baseURL string) *PropertiesEnricher {
	return &PropertiesEnricher{transport: transport, baseURL: baseURL}
}

func (e *PropertiesEnricher) ID() string { return "properties" }

func (e *PropertiesEnricher) Apply(ctx context.Context, c *card.Card) error {
	owner, repo, ok := strings.Cut(c.UID.String(), "/")
	if !ok || owner == "" || repo == "" {
		return &retrying.AbortError{Err: errBrokenUID(c.UID.String())}
	}
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/properties/values"
	resp, err := e.transport.Get(ctx, e.baseURL, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "github properties"}
	}
	props, err := propertiesFromBody(resp.Body)
	if err != nil {
		return err
	}
	c.SetProperties(props)
	return nil
}

func errBrokenUID(uid string) error {
	return &brokenUID{uid: uid}
}

type brokenUID struct{ uid string }

func (e *brokenUID) Error() string { return "broken github repository uid " + e.uid }

func propertiesFromBody(body []byte) (map[string]any, error) {
	var items []struct {
		Name  string          `json:"property_name"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	out := card.EmptyProperties()
	for _, it := range items {
		if it.Name == "" {
			continue
		}
		v, ok := coerceGitHubProperty(it.Value)
		if !ok {
			delete(out, it.Name)
			continue
		}
		out[it.Name] = v
	}
	return out, nil
}
