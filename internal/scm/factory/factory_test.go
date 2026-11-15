package factory

import (
	"net/http"
	"testing"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
	"github.com/Snowbridge/git-herder/internal/scm/conn"
)

func TestNewClientFactoryTransport(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(&http.Transport{})
	if f == nil {
		t.Fatal("nil factory")
	}
}

func TestNewListingUpstream(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(nil)
	cfg := conn.Config{Upstream: resolved.UpstreamGitHub, Host: "https://api.github.com"}
	crit := scm.ListingCriteria{
		Names: []string{"x"},
		Paths: []string{"y"},
		Tags:  []string{"z"},
		Attrs: []scm.NameValue{{Name: "k", Value: "v"}},
	}
	_, err := f.NewListing(cfg, crit)
	if err != nil {
		t.Fatalf("github: %v", err)
	}
	_, err = f.NewListing(conn.Config{Upstream: resolved.UpstreamGitLab, Host: "https://gitlab.com/api/v4"}, scm.ListingCriteria{})
	if err != nil {
		t.Fatalf("gitlab: %v", err)
	}
	_, err = f.NewListing(conn.Config{Upstream: resolved.UpstreamUnset}, crit)
	if err == nil {
		t.Fatal("expected unsupported upstream")
	}
}

func TestNewRepositoryCreatorUpstream(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(nil)
	c, err := f.NewRepositoryCreator(conn.Config{Upstream: resolved.UpstreamGitHub, Host: "https://api.github.com"})
	if err != nil || c == nil {
		t.Fatalf("github creator %v err %v", c, err)
	}
	c, err = f.NewRepositoryCreator(conn.Config{Upstream: resolved.UpstreamGitLab, Host: "https://gitlab.com"})
	if err != nil || c == nil {
		t.Fatalf("gitlab creator %v err %v", c, err)
	}
	if _, err = f.NewRepositoryCreator(conn.Config{Upstream: resolved.UpstreamUnset}); err == nil {
		t.Fatal("expected unsupported upstream")
	}
}

func TestNewListingGitHubNeedsQFragments(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(nil)
	_, err := f.NewListing(
		conn.Config{Upstream: resolved.UpstreamGitHub, Host: "https://api.github.com"},
		scm.ListingCriteria{Tags: []string{"only-tag"}},
	)
	if err != nil {
		t.Fatal(err)
	}
}
