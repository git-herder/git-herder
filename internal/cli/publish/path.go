package publish

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/workspace"
)

func parseRoot(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("root is required")
	}
	var out []string
	for _, seg := range strings.Split(raw, "/") {
		if seg == "" {
			continue
		}
		if seg == "." || seg == ".." {
			return nil, fmt.Errorf("invalid root %q", raw)
		}
		out = append(out, seg)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("invalid root %q", raw)
	}
	return out, nil
}

func flatSCM(u resolved.Upstream) bool {
	return u == resolved.UpstreamGitHub
}

// placed — путь репозитория в SCM и строка для итога.
type placed struct {
	Path string
	Name string
	Attr string
	Rel  string
}

func place(workspace, dir string, root []string, flat, leaf bool) (placed, error) {
	workspace = filepath.Clean(workspace)
	dir = filepath.Clean(dir)
	segs, err := relSegments(workspace, dir)
	if err != nil {
		return placed{}, err
	}
	rel := "."
	if len(segs) > 0 {
		rel = filepath.Join(segs...)
	}
	base := filepath.Base(dir)
	if dir == workspace {
		base = filepath.Base(workspace)
	}
	attr := "."
	if len(segs) > 0 {
		attr = strings.Join(segs, "/")
	}
	if flat {
		name := base
		if !leaf && len(segs) > 0 {
			name = strings.Join(segs, "-")
		}
		if len(root) == 0 {
			return placed{}, fmt.Errorf("root is required")
		}
		return placed{Path: root[0], Name: name, Attr: attr, Rel: rel}, nil
	}
	var mids []string
	if len(segs) > 0 {
		mids = segs[:len(segs)-1]
	}
	groups := append(append([]string{}, root...), mids...)
	return placed{Path: strings.Join(groups, "/"), Name: base, Attr: attr, Rel: rel}, nil
}

func relSegments(workspace, dir string) ([]string, error) {
	rel, err := filepath.Rel(workspace, dir)
	if err != nil {
		return nil, err
	}
	if rel == "." {
		return nil, nil
	}
	parts := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' })
	var out []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return nil, fmt.Errorf("copy %s is outside workspace", dir)
		}
		out = append(out, p)
	}
	return out, nil
}

func fullPath(p placed) string {
	if p.Path == "" {
		return p.Name
	}
	return p.Path + "/" + p.Name
}

func remoteAddress(host workspace.Host, full, protocol string, upstream resolved.Upstream, token string) (string, error) {
	page := "https://" + host.Name
	if host.Port != "" {
		page += ":" + host.Port
	}
	page += "/" + strings.TrimLeft(full, "/")
	addr, err := protocolURL(page, protocol)
	if err != nil {
		return "", err
	}
	if protocol != "https" || strings.TrimSpace(token) == "" {
		return addr, nil
	}
	user := "oauth2"
	if upstream == resolved.UpstreamGitHub {
		user = "x-access-token"
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, token)
	return u.String(), nil
}

func protocolURL(page, protocol string) (string, error) {
	u, err := url.Parse(page)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid repository url %q", page)
	}
	if protocol == "https" {
		return page, nil
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", fmt.Errorf("invalid repository url %q", page)
	}
	if !strings.HasSuffix(path, ".git") {
		path += ".git"
	}
	if port := u.Port(); port != "" {
		return "ssh://git@" + u.Hostname() + ":" + port + "/" + path, nil
	}
	return "git@" + u.Hostname() + ":" + path, nil
}

func duplicateName(items []placed) error {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if strings.EqualFold(items[i].Name, items[j].Name) {
				return fmt.Errorf("duplicate repository name %q", items[i].Name)
			}
		}
	}
	return nil
}
