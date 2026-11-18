// Подстановка {name} и {path} в хвосте прокси-команды.
// Шаблон хвоста не меняется: на каждую копию строится свой хвост.
package proxy

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	markerName        = "{name}"
	markerPath        = "{path}"
	emptyRelativePath = "empty relative path"
)

// copyMarkers считает имя каталога и относительный путь копии.
// У копии-корня путь пуст, имя — последний сегмент абсолютного workspace.
func copyMarkers(workspace, dir string) (name, path string, err error) {
	rel, err := filepath.Rel(workspace, dir)
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return filepath.Base(workspace), "", nil
	}
	parts := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' })
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", "", fmt.Errorf("copy %s is outside workspace", dir)
		}
		segs = append(segs, p)
	}
	if len(segs) == 0 {
		return filepath.Base(workspace), "", nil
	}
	return segs[len(segs)-1], strings.Join(segs, "/"), nil
}

func tailHasPath(tail []string) bool {
	for _, tok := range tail {
		if strings.Contains(tok, markerPath) {
			return true
		}
	}
	return false
}

// expandTail раскрывает маркеры из исходного токена за один проход.
func expandTail(tail []string, name, path string) []string {
	if len(tail) == 0 {
		return tail
	}
	out := make([]string, len(tail))
	for i, tok := range tail {
		out[i] = expandToken(tok, name, path)
	}
	return out
}

func expandToken(tok, name, path string) string {
	if !strings.Contains(tok, markerName) && !strings.Contains(tok, markerPath) {
		return tok
	}
	var b strings.Builder
	b.Grow(len(tok) + len(name) + len(path))
	for i := 0; i < len(tok); {
		switch {
		case strings.HasPrefix(tok[i:], markerName):
			b.WriteString(name)
			i += len(markerName)
		case strings.HasPrefix(tok[i:], markerPath):
			b.WriteString(path)
			i += len(markerPath)
		default:
			b.WriteByte(tok[i])
			i++
		}
	}
	return b.String()
}
