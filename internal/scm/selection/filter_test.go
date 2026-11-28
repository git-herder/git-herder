package selection

import (
	"context"
	"testing"

	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
)

func TestFilterGitHubAndGitLab(t *testing.T) {
	t.Parallel()
	cards := []card.Card{
		{Name: "api", Path: "group/sub", Labels: []string{"keep"}, Properties: map[string]any{"tier": "gold", "n": "2"}},
		{Name: "web", Path: "group/sub", Labels: []string{"drop"}, Properties: map[string]any{"tier": "silver"}},
	}
	gh := Filter(resolved.UpstreamGitHub, cards, scm.Criteria{
		Tags:  []scm.TagFilter{{Name: "keep"}, {Name: "nope", Absent: true}},
		Attrs: []scm.AttrFilter{{Key: "tier", Op: "=", Value: "ignored"}, {Key: "n", Op: ">", Value: "1"}},
	})
	if len(gh) != 1 || gh[0].Name != "api" {
		t.Fatalf("%+v", gh)
	}
	gl := Filter(resolved.UpstreamGitLab, cards, scm.Criteria{
		Paths: []string{"group/"},
		Names: []string{"api"},
		Tags:  []scm.TagFilter{{Name: "keep"}, {Name: "drop", Absent: true}},
		Attrs: []scm.AttrFilter{{Key: "missing", Op: "!=", Value: "x"}},
	})
	if len(gl) != 0 {
		t.Fatalf("missing key should drop, got %+v", gl)
	}
	gl = Filter(resolved.UpstreamGitLab, []card.Card{{Name: "api", Path: "a[b"}}, scm.Criteria{
		Paths: []string{"a["},
		Names: []string{"api"},
	})
	if len(gl) != 1 {
		t.Fatalf("substring fallback %+v", gl)
	}
	if got := Filter(resolved.UpstreamGitHub, cards, scm.Criteria{Search: true}); len(got) != 2 {
		t.Fatal("search skips filter")
	}
	ops := Filter(resolved.UpstreamGitHub, cards[:1], scm.Criteria{Attrs: []scm.AttrFilter{
		{Key: "tier", Op: "<=", Value: "gold"},
		{Key: "tier", Op: ">=", Value: "gold"},
		{Key: "tier", Op: "!=", Value: "silver"},
		{Key: "tier", Op: "<", Value: "h"},
	}})
	if len(ops) != 1 {
		t.Fatal(ops)
	}
	bad := Filter(resolved.UpstreamGitHub, cards[:1], scm.Criteria{Attrs: []scm.AttrFilter{{Key: "tier", Op: "~", Value: "x"}}})
	if len(bad) != 0 {
		t.Fatal("unknown op")
	}
	branched := []card.Card{
		{Name: "api", Branches: []string{"main", "feature/foo"}},
		{Name: "web", Branches: []string{"Main"}},
	}
	got := Filter(resolved.UpstreamGitHub, branched, scm.Criteria{Branches: []string{"feature/foo", "main"}})
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("%+v", got)
	}
	early := filterEarly(resolved.UpstreamGitLab, branched, scm.Criteria{
		Names:    []string{"api"},
		Branches: []string{"missing"},
	}, []scm.Enricher{idJob("branches")})
	if len(early) != 1 || early[0].Name != "api" {
		t.Fatalf("early keeps name match before branches, %+v", early)
	}
	late := filterLate(resolved.UpstreamGitLab, early, scm.Criteria{Branches: []string{"missing"}}, []scm.Enricher{idJob("branches")})
	if len(late) != 0 {
		t.Fatal("late drops missing branch")
	}
}

type idJob string

func (j idJob) ID() string { return string(j) }

func (idJob) Apply(context.Context, *card.Card) error { return nil }
