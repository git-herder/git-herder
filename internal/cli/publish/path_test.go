package publish

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/workspace"
)

func TestGitHostOf(t *testing.T) {
	t.Parallel()
	got, err := workspace.HostOf("https://api.github.com")
	if err != nil || got.Name != "github.com" || got.Port != "" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = workspace.HostOf("https://gitlab.example:8443/api/v4")
	if err != nil || got.Name != "gitlab.example" || got.Port != "8443" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = workspace.HostOf("https://ghe.example/api/v3")
	if err != nil || got.Name != "ghe.example" || got.Port != "" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = workspace.HostOf("://"); err == nil {
		t.Fatal("expected invalid host")
	}
}

func TestPlace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	nested := filepath.Join(ws, "domain", "billing", "api")
	got, err := place(ws, nested, []string{"acme"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "acme" || got.Name != "domain-billing-api" || got.Attr != "domain/billing/api" {
		t.Fatalf("%+v", got)
	}
	if got.Rel != filepath.Join("domain", "billing", "api") {
		t.Fatalf("rel %q", got.Rel)
	}
	leaf, err := place(ws, nested, []string{"acme"}, true, true)
	if err != nil || leaf.Name != "api" {
		t.Fatalf("%+v %v", leaf, err)
	}
	rootCopy, err := place(ws, ws, []string{"acme"}, true, false)
	if err != nil || rootCopy.Rel != "." || rootCopy.Attr != "." || rootCopy.Name != filepath.Base(ws) {
		t.Fatalf("%+v %v", rootCopy, err)
	}
	gl, err := place(ws, nested, []string{"group", "sub"}, false, false)
	if err != nil || gl.Path != "group/sub/domain/billing" || gl.Name != "api" {
		t.Fatalf("%+v %v", gl, err)
	}
	if _, err = parseRoot("group//./sub"); err == nil {
		t.Fatal("dot segment")
	}
	segs, err := parseRoot("/group//sub/")
	if err != nil || len(segs) != 2 || segs[0] != "group" || segs[1] != "sub" {
		t.Fatalf("%v %v", segs, err)
	}
	if !flatSCM(resolved.UpstreamGitHub) || flatSCM(resolved.UpstreamGitLab) {
		t.Fatal("topology")
	}
}

func TestDuplicateAndAddress(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	a, err := place(ws, filepath.Join(ws, "a", "b-c"), []string{"acme"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := place(ws, filepath.Join(ws, "a-b", "c"), []string{"acme"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "a-b-c" || b.Name != "a-b-c" {
		t.Fatalf("%s %s", a.Name, b.Name)
	}
	if err := duplicateName([]placed{a, b}); err == nil {
		t.Fatal("expected duplicate")
	}
	upper := a
	upper.Name = "A-B-C"
	if err := duplicateName([]placed{a, upper}); err == nil {
		t.Fatal("expected case-insensitive duplicate")
	}
	host, err := workspace.HostOf("https://api.github.com")
	if err != nil {
		t.Fatal(err)
	}
	addr, err := remoteAddress(host, fullPath(a), "ssh", resolved.UpstreamGitHub, "")
	if err != nil || addr != "git@github.com:acme/a-b-c.git" {
		t.Fatalf("%q %v", addr, err)
	}
	httpsAddr, err := remoteAddress(host, fullPath(a), "https", resolved.UpstreamGitHub, "secret")
	if err != nil || httpsAddr != "https://x-access-token:secret@github.com/acme/a-b-c" {
		t.Fatalf("%q %v", httpsAddr, err)
	}
	glHost := workspace.Host{Name: "gitlab.example", Port: "2222"}
	gl, err := remoteAddress(glHost, "group/sub/api", "ssh", resolved.UpstreamGitLab, "secret")
	if err != nil || gl != "ssh://git@gitlab.example:2222/group/sub/api.git" {
		t.Fatalf("%q %v", gl, err)
	}
	plain, err := remoteAddress(workspace.Host{Name: "gitlab.example"}, "group/api", "https", resolved.UpstreamGitLab, "secret")
	if err != nil || !strings.Contains(plain, "oauth2:secret@gitlab.example/group/api") {
		t.Fatalf("%q %v", plain, err)
	}
}
