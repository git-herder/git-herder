package conn

import (
	"testing"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/scm"
)

func TestFromGlobals(t *testing.T) {
	t.Parallel()
	g := resolved.Globals{
		Upstream: resolved.UpstreamGitHub,
		Host:     "https://api.github.com",
		Token:    "t",
	}
	c := FromGlobals(g, 0, true)
	if c.PerPage != scm.DefaultPerPage || c.Host != g.Host || !c.IncludeInactive {
		t.Fatalf("%+v", c)
	}
}
