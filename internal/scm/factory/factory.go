// Пакет factory: ScmClientFactory — выбор GitHub/GitLab listing и enrichment по upstream.
package factory

import (
	"fmt"
	"net/http"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
	"github.com/Snowbridge/git-herder/internal/scm/github"
	"github.com/Snowbridge/git-herder/internal/scm/gitlab"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// ClientFactory создаёт listing-адаптеры по conn.Config и критериям.
type ClientFactory struct {
	transport *scmhttp.Transport
}

// NewClientFactory создаёт фабрику с HTTP-клиентом (Transport опционален).
func NewClientFactory(httpTransport *http.Transport) *ClientFactory {
	client := &http.Client{}
	if httpTransport != nil {
		client.Transport = httpTransport
	}
	return NewClientFactoryWithClient(client)
}

// NewClientFactoryWithClient использует готовый HTTP-клиент (тесты httptest).
func NewClientFactoryWithClient(client *http.Client) *ClientFactory {
	if client == nil {
		client = &http.Client{}
	}
	return &ClientFactory{
		transport: scmhttp.NewTransport(client, ""),
	}
}

// Open собирает listing и активные задания обогащения.
func (f *ClientFactory) Open(cfg conn.Config, crit scm.Criteria) (scm.Listing, []scm.Enricher, error) {
	t := f.scopedTransport(cfg)
	if crit.Search {
		switch cfg.Upstream {
		case resolved.UpstreamGitHub:
			l, err := github.NewSearchListing(t, cfg, crit.SearchPairs)
			return l, nil, err
		case resolved.UpstreamGitLab:
			l, err := gitlab.NewSearchListing(t, cfg, crit.SearchPairs)
			return l, nil, err
		default:
			return nil, nil, fmt.Errorf("unsupported upstream %d", cfg.Upstream)
		}
	}
	listing, err := f.buildQueryListing(t, cfg, crit)
	if err != nil {
		return nil, nil, err
	}
	return listing, f.enrichers(t, cfg, crit), nil
}

// NewListing настраивает билдер выбранного upstream и возвращает ScmListing.
func (f *ClientFactory) NewListing(cfg conn.Config, crit scm.ListingCriteria) (scm.Listing, error) {
	c := scm.Criteria{
		Paths:           crit.Paths,
		Names:           crit.Names,
		Scope:           cfg.SelectionScope,
		IncludeInactive: cfg.IncludeInactive,
	}
	for _, tag := range crit.Tags {
		c.Tags = append(c.Tags, scm.TagFilter{Name: tag})
	}
	for _, attr := range crit.Attrs {
		c.Attrs = append(c.Attrs, scm.AttrFilter{Key: attr.Name, Op: "=", Value: attr.Value})
	}
	listing, _, err := f.Open(cfg, c)
	return listing, err
}

func (f *ClientFactory) scopedTransport(cfg conn.Config) *scmhttp.Transport {
	t := scmhttp.NewTransport(f.transport.Client(), cfg.Token)
	if cfg.Upstream != resolved.UpstreamGitHub {
		return t
	}
	policy := scmhttp.LimitAwait
	if cfg.OnLimit == "abort" {
		policy = scmhttp.LimitAbort
	}
	t.SetQuota(scmhttp.ParseGitHubQuota, policy, cfg.OnQuotaWait)
	return t
}

func (f *ClientFactory) buildQueryListing(t *scmhttp.Transport, cfg conn.Config, crit scm.Criteria) (scm.Listing, error) {
	var b scm.ListingBuilder
	switch cfg.Upstream {
	case resolved.UpstreamGitHub:
		b = github.NewListingBuilder(t, cfg)
	case resolved.UpstreamGitLab:
		b = gitlab.NewListingBuilder(t, cfg)
	default:
		return nil, fmt.Errorf("unsupported upstream %d", cfg.Upstream)
	}
	for _, name := range crit.Names {
		b = b.SetName(name)
	}
	for _, path := range crit.Paths {
		b = b.SetPath(path)
	}
	for _, tag := range crit.PresenceTags() {
		b = b.SetTag(tag)
	}
	for _, attr := range crit.EqualAttrs() {
		b = b.SetAttribute(attr.Name, attr.Value)
	}
	return b.Build()
}

func (f *ClientFactory) enrichers(t *scmhttp.Transport, cfg conn.Config, crit scm.Criteria) []scm.Enricher {
	var jobs []scm.Enricher
	switch cfg.Upstream {
	case resolved.UpstreamGitHub:
		if len(crit.Attrs) > 0 {
			jobs = append(jobs, github.NewPropertiesEnricher(t, cfg.Host))
		}
	case resolved.UpstreamGitLab:
		if len(crit.Tags) > 0 {
			jobs = append(jobs, gitlab.NewLabelsEnricher(t, cfg.Host))
		}
		if len(crit.Attrs) > 0 {
			jobs = append(jobs, gitlab.NewPropertiesEnricher(t, cfg.Host))
		}
	}
	return jobs
}
