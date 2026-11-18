// Разбор argv прокси-команд: префикс флагов Git Herder и хвост для git.
// Склейка коротких флагов начинает хвост — это исключение из pflag.
package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ProxyCall — разобранный запуск pull, push, fetch или remote.
type ProxyCall struct {
	Command   string
	Tail      []string
	AllTokens []string
	Help      bool
	Version   bool
}

// FlagValueError — у флага префикса нет отдельного значения. Код выхода 2.
type FlagValueError struct {
	Token string
}

func (e *FlagValueError) Error() string {
	return "flag needs a value: " + e.Token
}

// FlagSyntaxError — значение логического флага префикса не разбирается. Код выхода 2.
type FlagSyntaxError struct {
	Token string
}

func (e *FlagSyntaxError) Error() string {
	return "invalid flag value: " + e.Token
}

type proxyCallKey struct{}

// WithProxyCall кладёт разбор в контекст команды.
func WithProxyCall(ctx context.Context, call ProxyCall) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, proxyCallKey{}, call)
}

// ProxyCallFrom читает разбор, если bootstrap его положил.
func ProxyCallFrom(ctx context.Context) (ProxyCall, bool) {
	if ctx == nil {
		return ProxyCall{}, false
	}
	call, ok := ctx.Value(proxyCallKey{}).(ProxyCall)
	return call, ok
}

type flagSpec struct {
	value   bool
	all     bool
	help    bool
	version bool
}

var longPrefixFlags = map[string]flagSpec{
	"verbose":         {},
	"debug":           {},
	"retries-number":  {value: true},
	"on-error":        {value: true},
	"workspace":       {value: true},
	"upstream":        {value: true},
	"host":            {value: true},
	"token":           {value: true},
	"git-https-token": {value: true},
	"profile":         {value: true},
	"version":         {version: true},
	"help":            {help: true},
	"qn":              {value: true},
	"qp":              {value: true},
	"interactive":     {},
	"all":             {all: true},
}

var shortPrefixFlags = map[byte]flagSpec{
	'V': {},
	'D': {},
	'R': {value: true},
	'e': {value: true},
	'w': {value: true},
	'U': {value: true},
	'H': {value: true},
	't': {value: true},
	'v': {version: true},
	'h': {help: true},
	'i': {},
	'A': {all: true},
}

var proxyCommandNames = map[string]struct{}{
	"pull":   {},
	"push":   {},
	"fetch":  {},
	"remote": {},
}

// PrepareProxyArgs делит argv прокси-команды на флаги cobra и хвост.
// Если это не pull, push, fetch или remote, args возвращаются как есть.
// Перед хвостом вставляется «--», чтобы pflag не забрал аргументы git.
func PrepareProxyArgs(args []string) ([]string, *ProxyCall, error) {
	call, prefix, err := scanProxyArgs(args)
	if err != nil {
		return nil, nil, err
	}
	if call == nil {
		return args, nil, nil
	}
	out := make([]string, 0, 1+len(prefix)+1+len(call.Tail))
	out = append(out, call.Command)
	out = append(out, prefix...)
	if len(call.Tail) > 0 {
		out = append(out, "--")
		out = append(out, call.Tail...)
	}
	return out, call, nil
}

func scanProxyArgs(args []string) (*ProxyCall, []string, error) {
	var (
		command   string
		prefix    []string
		allOn     []string
		helpAt    = -1
		versionAt = -1
		pending   error
	)
	finish := func(tail []string) (*ProxyCall, []string, error) {
		if pending != nil {
			return nil, nil, pending
		}
		call := &ProxyCall{
			Command:   command,
			Tail:      tail,
			AllTokens: allOn,
		}
		switch {
		case helpAt >= 0 && (versionAt < 0 || helpAt < versionAt):
			call.Help = true
		case versionAt >= 0:
			call.Version = true
		}
		return call, prefix, nil
	}
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if tok == "--" {
			if command == "" {
				return nil, nil, nil
			}
			return finish(append([]string{}, args[i+1:]...))
		}
		spec, inline, hasInline, ok := classifyPrefix(tok)
		if !ok {
			if command == "" {
				if _, isCmd := proxyCommandNames[tok]; isCmd {
					command = tok
					if pending != nil {
						return nil, nil, pending
					}
					continue
				}
				return nil, nil, nil
			}
			return finish(append([]string{}, args[i:]...))
		}
		if spec.value && !hasInline {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				err := &FlagValueError{Token: tok}
				if command != "" {
					return nil, nil, err
				}
				if pending == nil {
					pending = err
				}
				continue
			}
			prefix = append(prefix, tok, args[i+1])
			i++
			continue
		}
		if !spec.value && hasInline {
			on, err := strconv.ParseBool(inline)
			if err != nil {
				bad := &FlagSyntaxError{Token: tok}
				if command != "" {
					return nil, nil, bad
				}
				if pending == nil {
					pending = bad
				}
				continue
			}
			noteBool(spec, tok, on, i, &allOn, &helpAt, &versionAt)
			prefix = append(prefix, tok)
			continue
		}
		noteBool(spec, tok, true, i, &allOn, &helpAt, &versionAt)
		prefix = append(prefix, tok)
	}
	if command == "" {
		return nil, nil, nil
	}
	return finish(nil)
}

func noteBool(spec flagSpec, tok string, on bool, at int, allOn *[]string, helpAt, versionAt *int) {
	if spec.all {
		if on {
			*allOn = append(*allOn, tok)
		}
	}
	if spec.help && on && *helpAt < 0 {
		*helpAt = at
	}
	if spec.version && on && *versionAt < 0 {
		*versionAt = at
	}
}

func classifyPrefix(tok string) (spec flagSpec, inline string, hasInline, ok bool) {
	if tok == "-" || !strings.HasPrefix(tok, "-") {
		return flagSpec{}, "", false, false
	}
	if strings.HasPrefix(tok, "--") {
		body := strings.TrimPrefix(tok, "--")
		name, val, has := strings.Cut(body, "=")
		if name == "" {
			return flagSpec{}, "", false, false
		}
		spec, ok = longPrefixFlags[name]
		if !ok {
			return flagSpec{}, "", false, false
		}
		return spec, val, has, true
	}
	body := tok[1:]
	if body == "" {
		return flagSpec{}, "", false, false
	}
	spec, ok = shortPrefixFlags[body[0]]
	if !ok {
		return flagSpec{}, "", false, false
	}
	rest := body[1:]
	if rest == "" {
		return spec, "", false, true
	}
	if strings.HasPrefix(rest, "=") {
		return spec, strings.TrimPrefix(rest, "="), true, true
	}
	return flagSpec{}, "", false, false
}

// FormatProxyNotice собирает строку о том, что --all не уходит в git.
func FormatProxyNotice(tokens []string) string {
	if len(tokens) == 0 {
		tokens = []string{"--all"}
	}
	return fmt.Sprintf("Not passed to git: %s", strings.Join(tokens, " "))
}
