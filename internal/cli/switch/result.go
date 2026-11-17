package switchcmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	signOK       = "✔️"
	signSame     = "➡️"
	signConflict = "⚔️"
	signFail     = "❌"
	signSkip     = "⏩"
)

func badSign(sign string) bool {
	return sign == signFail || sign == signConflict
}

func formatSwitch(sw, up, rel string, notes []string) string {
	var b strings.Builder
	b.WriteString(sw)
	b.WriteByte(' ')
	b.WriteString(up)
	b.WriteByte(' ')
	b.WriteString(rel)
	for _, note := range notes {
		if note == "" {
			continue
		}
		b.WriteByte('\n')
		b.WriteString("    ")
		b.WriteString(note)
	}
	return b.String()
}

func conflictNote(names []string) string {
	if len(names) == 0 {
		return ""
	}
	shown := names
	extra := 0
	if len(names) > 3 {
		shown = names[:3]
		extra = len(names) - 3
	}
	line := signConflict + " " + strings.Join(shown, ", ")
	if extra > 0 {
		line += fmt.Sprintf(" and %d more files", extra)
	}
	return line
}

func shortReason(out gitOut, fallback string) string {
	if line := firstNonEmpty(out.Stderr); line != "" {
		return line
	}
	if line := firstNonEmpty(out.Stdout); line != "" {
		return line
	}
	return fallback
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

func oneLine(s string) (string, bool) {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		return "", false
	}
	return lines[0], true
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

func launchFailed(out gitOut, err error) bool {
	return err != nil && out.Exit == 0
}
