// Пакет factory: ScmClientFactory — выбор GitHub/GitLab listing по upstream.
package factory

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/github"
	"github.com/Snowbridge/git-herder/internal/scm/gitlab"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

// ClientFactory создаёт listing-адаптеры по ConnConfig и критериям.
type ClientFactory struct {
	transport *scmhttp.Transport
}

// NewClientFactory создаёт фабрику с HTTP-клиентом (Transport опционален).
func NewClientFactory(httpTransport *http.Transport) *ClientFactory {
	client := &http.Client{}
	if httpTransport != nil {
		client.Transport = httpTransport
	}
	return &ClientFactory{
		transport: scmhttp.NewTransport(client, ""),
	}
}

// NewListing настраивает билдер выбранного upstream и возвращает ScmListing.
func (f *ClientFactory) NewListing(cfg scm.ConnConfig, crit scm.ListingCriteria) (scm.Listing, error) {
	t := scmhttp.NewTransport(f.transport.Client(), cfg.Token)
	var b scm.ListingBuilder
	switch strings.ToLower(strings.TrimSpace(cfg.Upstream)) {
	case "github":
		b = github.NewListingBuilder(t, cfg)
	case "gitlab":
		b = gitlab.NewListingBuilder(t, cfg)
	default:
		return nil, fmt.Errorf("unsupported upstream %q", cfg.Upstream)
	}
	for _, name := range crit.Names {
		b = b.SetName(name)
	}
	for _, path := range crit.Paths {
		b = b.SetPath(path)
	}
	for _, tag := range crit.Tags {
		b = b.SetTag(tag)
	}
	for _, attr := range crit.Attrs {
		b = b.SetAttribute(attr.Name, attr.Value)
	}
	return b.Build()
}
