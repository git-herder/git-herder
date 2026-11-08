package factory

import (
	"testing"

	"github.com/Snowbridge/git-herder/internal/scm"
)

func TestNewListingUpstream(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(nil)
	cfg := scm.ConnConfig{Upstream: "github", Host: "github.com"}
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
	cfg.Upstream = "gitlab"
	b := scm.ConnConfig{Upstream: "gitlab", Host: "gitlab.com"}
	_, err = f.NewListing(b, scm.ListingCriteria{})
	if err != nil {
		t.Fatalf("gitlab: %v", err)
	}
	cfg.Upstream = "svn"
	_, err = f.NewListing(cfg, crit)
	if err == nil {
		t.Fatal("expected unsupported upstream")
	}
}

func TestNewListingGitHubNeedsQFragments(t *testing.T) {
	t.Parallel()
	f := NewClientFactory(nil)
	_, err := f.NewListing(scm.ConnConfig{Upstream: "github", Host: "github.com"}, scm.ListingCriteria{Tags: []string{"only-tag"}})
	if err == nil {
		t.Fatal("expected empty q error")
	}
}
