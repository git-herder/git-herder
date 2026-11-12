package scm

import (
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/resolved"
)

func TestParseTagAndAttr(t *testing.T) {
	t.Parallel()
	tag, err := ParseTag("Foo=NuLl")
	if err != nil || !tag.Absent || tag.Name != "Foo" {
		t.Fatalf("%+v %v", tag, err)
	}
	if _, err = ParseTag("=null"); err == nil {
		t.Fatal("empty name")
	}
	if _, err = ParseTag("a=b"); err == nil {
		t.Fatal("bad eq")
	}
	if _, err = ParseTag("a<b"); err == nil {
		t.Fatal("lt")
	}
	if _, err = ParseTag(""); err == nil {
		t.Fatal("empty")
	}
	ok, err := ParseTag("docs")
	if err != nil || ok.Absent || ok.Name != "docs" {
		t.Fatal(ok, err)
	}
	attr, err := ParseAttr("tier>=gold")
	if err != nil || attr.Key != "tier" || attr.Op != ">=" || attr.Value != "gold" {
		t.Fatalf("%+v %v", attr, err)
	}
	if _, err = ParseAttr("novalue"); err == nil {
		t.Fatal("no op")
	}
	if _, err = ParseAttr("=x"); err == nil {
		t.Fatal("empty key")
	}
	pairs, err := ParseSearch("q=org:acme sort=updated q=org:other")
	if err != nil || len(pairs) != 2 || pairs[0].Value != "org:other" {
		t.Fatalf("%+v %v", pairs, err)
	}
	if _, err = ParseSearch("   "); err == nil {
		t.Fatal("blank")
	}
	if _, err = ParseSearch("novalue"); err == nil {
		t.Fatal("token")
	}
}

func TestValidateShape(t *testing.T) {
	t.Parallel()
	c := Criteria{Search: true, Paths: []string{"a"}, SearchPairs: []SearchPair{{Key: "q", Value: "x"}}}
	if err := c.ValidateShape(resolved.UpstreamGitHub); err == nil {
		t.Fatal("conflict")
	}
	gh := Criteria{IncludeInactiveSet: true}
	if err := gh.ValidateShape(resolved.UpstreamGitHub); err == nil {
		t.Fatal("inactive")
	}
	badLim := Criteria{OnLimitSet: true, OnLimit: "nope"}
	if err := badLim.ValidateShape(resolved.UpstreamGitHub); err == nil {
		t.Fatal("limit")
	}
	def := Criteria{Names: []string{"ok"}}
	if err := def.ValidateShape(resolved.UpstreamGitHub); err != nil || def.OnLimit != "await" {
		t.Fatal(err, def.OnLimit)
	}
	badQP := Criteria{Paths: []string{"a*b"}}
	if err := badQP.ValidateShape(resolved.UpstreamGitHub); err == nil {
		t.Fatal("qp")
	}
	glLim := Criteria{OnLimitSet: true, OnLimit: "await"}
	if err := glLim.ValidateShape(resolved.UpstreamGitLab); err == nil {
		t.Fatal("gitlab limit")
	}
	search := Criteria{Search: true, SearchPairs: []SearchPair{{Key: "sort", Value: "x"}}}
	if err := search.ValidateShape(resolved.UpstreamGitHub); err == nil {
		t.Fatal("missing q")
	}
	glSearch := Criteria{Search: true, SearchPairs: []SearchPair{{Key: "search", Value: "x"}}}
	if err := glSearch.ValidateShape(resolved.UpstreamGitLab); err != nil {
		t.Fatal(err)
	}
	if glSearch.SearchPairs[len(glSearch.SearchPairs)-1].Value != "projects" {
		t.Fatalf("%+v", glSearch.SearchPairs)
	}
	badScope := Criteria{Search: true, SearchPairs: []SearchPair{{Key: "scope", Value: "issues"}}}
	if err := badScope.ValidateShape(resolved.UpstreamGitLab); err == nil {
		t.Fatal("scope")
	}
	var bare Criteria
	if err := bare.ValidateShape(resolved.UpstreamUnset); err == nil {
		t.Fatal("upstream")
	}
	onlyQT := Criteria{Tags: []TagFilter{{Name: "t"}}}
	withEq := Criteria{Attrs: []AttrFilter{{Key: "k", Op: "=", Value: "v"}}}
	if onlyQT.GitHubHasQueryFragment() || !withEq.GitHubHasQueryFragment() {
		t.Fatal("q fragments")
	}
	if !withEq.HasCriterion() || !withEq.GitHubHasQueryFragment() {
		t.Fatal("criterion")
	}
	if len(Criteria{Tags: []TagFilter{{Name: "a"}, {Name: "b", Absent: true}}}.PresenceTags()) != 1 {
		t.Fatal("presence")
	}
	if len(withEq.EqualAttrs()) != 1 {
		t.Fatal("equals")
	}
}

func TestPublicSaaSHost(t *testing.T) {
	t.Parallel()
	if !PublicSaaSHost("https://gitlab.com/api/v4") || !PublicSaaSHost("github.com") {
		t.Fatal("expected public")
	}
	if PublicSaaSHost("https://api.github.com") || PublicSaaSHost("https://gitlab.example") {
		t.Fatal("not public")
	}
	if strings.TrimSpace("") != "" {
		t.Fatal("keep")
	}
}
