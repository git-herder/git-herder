// Строка итога прокси-команды: знак, относительный путь и текст ошибки.
package proxy

import (
	"path/filepath"
	"strings"

	"github.com/git-herder/git-herder/internal/gitexec"
)

const (
	signOK   = "✔️"
	signFail = "❌"
)

func formatResult(ok bool, rel, note string) string {
	sign := signOK
	if !ok {
		sign = signFail
	}
	if note == "" {
		return sign + " " + rel
	}
	return sign + " " + rel + "\n    " + note
}

func shortReason(res gitexec.Result) string {
	if line := firstNonEmpty(res.Stderr); line != "" {
		return line
	}
	if line := firstNonEmpty(res.Stdout); line != "" {
		return line
	}
	return "git failed"
}

func firstNonEmpty(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func relToWorkspace(root, dir string) (string, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return ".", nil
	}
	return filepath.ToSlash(rel), nil
}

func failedRun(res gitexec.Result, err error) bool {
	return err != nil || res.Exit != 0
}
