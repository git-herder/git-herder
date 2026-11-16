// Пакет redact маскирует секреты в строках и структурах данных (спека platform/logging.md).
package redact

import (
	"fmt"
	"regexp"
	"strings"
)

const mask = "***"

var (
	tokenFlag   = regexp.MustCompile(`(?i)(--token(?:=[^\s]+)?|-t(?:=[^\s]+)?\s+\S+|-t[^\s=]\S*)`)
	gitHTTPSArg = regexp.MustCompile(`(?i)--git-https-token(?:=|\s+)\S+`)
	userinfo    = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)([^/\s@]+)@`)
	secretKeys  = map[string]struct{}{
		"token":           {},
		"git-https-token": {},
		"git_https_token": {},
		"password":        {},
		"private_token":   {},
		"pat":             {},
		"authorization":   {},
		"private-key":     {},
		"private_key":     {},
	}
)

// CommandLine маскирует флаги --token, -t и --git-https-token в командной строке.
func CommandLine(argv string) string {
	const placeholder = "\x00githttps\x00"
	out := gitHTTPSArg.ReplaceAllString(argv, placeholder)
	out = tokenFlag.ReplaceAllString(out, "--token="+mask)
	out = regexp.MustCompile(`(?i)-t\s+\S+`).ReplaceAllString(out, "-t "+mask)
	return strings.ReplaceAll(out, placeholder, "--git-https-token="+mask)
}

// String заменяет известные фрагменты Authorization и query token=.
func String(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = regexp.MustCompile(`(?i)(Authorization[=:]\s*)(\S+)`).ReplaceAllString(out, `${1}`+mask)
	out = regexp.MustCompile(`(?i)([?&](?:token|password|private_token)=)[^&\s]+`).ReplaceAllString(out, `${1}`+mask)
	out = userinfo.ReplaceAllString(out, `${1}`+mask+`@`)
	return out
}

// MapYAML рекурсивно маскирует чувствительные ключи в дереве YAML (map/slice).
func MapYAML(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if isSecretKey(k) {
				out[k] = mask
				continue
			}
			out[k] = MapYAML(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				out[fmtKey(k)] = MapYAML(val)
				continue
			}
			if isSecretKey(ks) {
				out[ks] = mask
				continue
			}
			out[ks] = MapYAML(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = MapYAML(item)
		}
		return out
	default:
		return v
	}
}

// IsSecretKey сообщает, что имя поля или заголовка несёт секрет.
func IsSecretKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	if _, ok := secretKeys[k]; ok {
		return true
	}
	_, ok := secretKeys[strings.ReplaceAll(k, "-", "_")]
	return ok
}

func isSecretKey(k string) bool {
	return IsSecretKey(k)
}

func fmtKey(k any) string {
	return strings.TrimSpace(strings.ToLower(strings.ReplaceAll(fmt.Sprint(k), " ", "")))
}
