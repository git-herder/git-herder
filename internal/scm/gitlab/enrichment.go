package gitlab

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/card"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// LabelsEnricher догружает labels через GET /projects/:id/labels.
type LabelsEnricher struct {
	transport *scmhttp.Transport
	baseURL   string
}

// PropertiesEnricher догружает properties через GET /projects/:id/custom_attributes.
type PropertiesEnricher struct {
	transport *scmhttp.Transport
	baseURL   string
}

// NewLabelsEnricher создаёт задание обогащения labels.
func NewLabelsEnricher(transport *scmhttp.Transport, baseURL string) *LabelsEnricher {
	return &LabelsEnricher{transport: transport, baseURL: baseURL}
}

// NewPropertiesEnricher создаёт задание обогащения properties.
func NewPropertiesEnricher(transport *scmhttp.Transport, baseURL string) *PropertiesEnricher {
	return &PropertiesEnricher{transport: transport, baseURL: baseURL}
}

func (e *LabelsEnricher) ID() string { return "labels" }

func (e *LabelsEnricher) Apply(ctx context.Context, c *card.Card) (err error) {
	fullName := ""
	if c != nil {
		fullName = c.FullName()
	}
	leave := logging.Enter("ctx", ctx, "id", e.ID(), "fullName", fullName)
	defer func() { leave("err", err) }()
	id, ok := c.UID.Number()
	if !ok {
		return &scm.HTTPError{Status: 0, What: "gitlab labels: uid is not numeric"}
	}
	path := apiV4 + "/projects/" + strconv.FormatInt(id, 10) + "/labels"
	resp, err := e.transport.Get(ctx, e.baseURL, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "gitlab labels"}
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	labels := make([]string, 0, len(items))
	for _, it := range items {
		if it.Name == "" {
			continue
		}
		if _, ok := seen[it.Name]; ok {
			continue
		}
		seen[it.Name] = struct{}{}
		labels = append(labels, it.Name)
	}
	c.SetLabels(labels)
	return nil
}

func (e *PropertiesEnricher) ID() string { return "properties" }

func (e *PropertiesEnricher) Apply(ctx context.Context, c *card.Card) (err error) {
	fullName := ""
	if c != nil {
		fullName = c.FullName()
	}
	leave := logging.Enter("ctx", ctx, "id", e.ID(), "fullName", fullName)
	defer func() { leave("err", err) }()
	id, ok := c.UID.Number()
	if !ok {
		return &scm.HTTPError{Status: 0, What: "gitlab properties: uid is not numeric"}
	}
	path := apiV4 + "/projects/" + strconv.FormatInt(id, 10) + "/custom_attributes"
	resp, err := e.transport.Get(ctx, e.baseURL, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return &scm.HTTPError{Status: resp.Status, What: "gitlab properties"}
	}
	var items []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return err
	}
	props := card.EmptyProperties()
	for _, it := range items {
		if it.Key == "" {
			continue
		}
		props[it.Key] = coerceText(it.Value)
	}
	c.SetProperties(props)
	return nil
}
