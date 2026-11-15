package clone

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// errNoRemainder — полное имя целиком совпало с хвостом workspace.
var errNoRemainder = fmt.Errorf("destination path is empty after overlap")

// errParentEscape — в значении атрибута есть сегмент «..».
var errParentEscape = fmt.Errorf("attribute path contains ..")

// attributeDestination собирает каталог из значения пользовательского атрибута.
// raw делится по / и \. Пустые сегменты и «.» отбрасываются. «..» — ошибка.
// preserve добавляет сегменты path карточки перед родителями из атрибута.
func attributeDestination(workspace, repoPath, name, raw string, preserve bool) (string, error) {
	parents, err := splitAttribute(raw)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", errNoRemainder
	}
	parts := []string{workspace}
	if preserve {
		parts = append(parts, splitRepo(repoPath)...)
	}
	parts = append(parts, parents...)
	parts = append(parts, name)
	return filepath.Join(parts...), nil
}

func splitAttribute(raw string) ([]string, error) {
	segs := strings.FieldsFunc(raw, isSep)
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		if s == "" || s == "." {
			continue
		}
		if s == ".." {
			return nil, errParentEscape
		}
		out = append(out, s)
	}
	return out, nil
}

func destination(workspace, fullName string, preserve bool) (string, error) {
	repo := splitRepo(fullName)
	if len(repo) == 0 {
		return "", errNoRemainder
	}
	k := 0
	if !preserve {
		k = overlap(splitWorkspace(workspace), repo)
	}
	rest := repo[k:]
	if len(rest) == 0 {
		return "", errNoRemainder
	}
	parts := append([]string{workspace}, rest...)
	return filepath.Join(parts...), nil
}

func splitWorkspace(p string) []string {
	vol := filepath.VolumeName(p)
	rest := strings.TrimPrefix(p, vol)
	return strings.FieldsFunc(rest, isSep)
}

func splitRepo(name string) []string {
	return strings.FieldsFunc(name, func(r rune) bool { return r == '/' })
}

func isSep(r rune) bool {
	return r == '/' || r == '\\'
}

func overlap(ws, repo []string) int {
	max := len(ws)
	if len(repo) < max {
		max = len(repo)
	}
	best := 0
	for k := 1; k <= max; k++ {
		if equalSeg(ws[len(ws)-k:], repo[:k]) {
			best = k
		}
	}
	return best
}

func equalSeg(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func repoURL(page, protocol string) (string, error) {
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
