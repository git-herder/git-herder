// Пакет cli: резолв глобальных параметров argv + профиль (Invocation).
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Snowbridge/git-herder/internal/resolved"
)

// BuiltinDefaults встроенные значения из cli/index.md.
func BuiltinDefaults() resolved.Globals {
	wd, _ := os.Getwd()
	d := resolved.BuiltinDefaults()
	d.Workspace = wd
	return d
}

// ResolveGlobals вычисляет итоговые значения; profileDoc nil — профиль не загружен.
func ResolveGlobals(cmd *cobra.Command, profileDoc map[string]any, leafCommandName string) (resolved.Globals, error) {
	out := BuiltinDefaults()
	flags := cmd.Root().PersistentFlags()

	verbose, _ := flags.GetBool("verbose")
	out.Verbose = verbose

	debug, err := resolveBoolFlag(flags, "debug", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	out.Debug = debug

	retries, err := resolveIntFlag(flags, "retries-number", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	if retries >= 0 {
		out.RetriesNumber = retries
	}

	onError, err := resolveStringFlag(flags, "on-error", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	if onError != "" {
		parsed, err := resolved.ParseOnError(onError)
		if err != nil {
			return out, err
		}
		out.OnError = parsed
	}

	ws, err := resolveStringFlag(flags, "workspace", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	if ws != "" {
		abs, err := absolutizePath(ws)
		if err != nil {
			return out, err
		}
		out.Workspace = abs
	}

	upstream, err := resolveStringFlag(flags, "upstream", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	if strings.TrimSpace(upstream) != "" {
		out.Upstream, err = resolved.ParseUpstream(upstream)
		if err != nil {
			return out, err
		}
	}

	host, err := resolveStringFlag(flags, "host", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	out.Host = strings.TrimSpace(host)
	if out.Host == "" && out.Upstream != resolved.UpstreamUnset {
		out.Host = resolved.DefaultHost(out.Upstream)
	}

	token, err := resolveStringFlag(flags, "token", cmd, profileDoc, leafCommandName)
	if err != nil {
		return out, err
	}
	out.Token = token

	return out, nil
}

func resolveBoolFlag(flags *pflag.FlagSet, name string, cmd *cobra.Command, doc map[string]any, leaf string) (bool, error) {
	if flags.Changed(name) {
		v, err := flags.GetBool(name)
		return v, err
	}
	if v, ok := lookupProfileValue(doc, cmd, leaf, name); ok {
		switch x := v.(type) {
		case bool:
			return x, nil
		case string:
			return x == "true", nil
		}
	}
	v, _ := flags.GetBool(name)
	return v, nil
}

func resolveIntFlag(flags *pflag.FlagSet, name string, cmd *cobra.Command, doc map[string]any, leaf string) (int, error) {
	if flags.Changed(name) {
		return flags.GetInt(name)
	}
	if v, ok := lookupProfileValue(doc, cmd, leaf, name); ok {
		switch x := v.(type) {
		case int:
			return x, nil
		case int64:
			return int(x), nil
		case float64:
			return int(x), nil
		}
	}
	return flags.GetInt(name)
}

func resolveStringFlag(flags *pflag.FlagSet, name string, cmd *cobra.Command, doc map[string]any, leaf string) (string, error) {
	if flags.Changed(name) {
		return flags.GetString(name)
	}
	if v, ok := lookupProfileValue(doc, cmd, leaf, name); ok {
		return fmt.Sprint(v), nil
	}
	s, err := flags.GetString(name)
	return s, err
}

func lookupProfileValue(doc map[string]any, cmd *cobra.Command, leaf, key string) (any, bool) {
	if doc == nil {
		return nil, false
	}
	if block, ok := doc[leaf].(map[string]any); ok {
		if v, ok := block[key]; ok {
			return v, true
		}
	}
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		if p.Name() == "" {
			continue
		}
		if block, ok := doc[p.Name()].(map[string]any); ok {
			if v, ok := block[key]; ok {
				return v, true
			}
		}
	}
	if root, ok := doc["root"].(map[string]any); ok {
		if v, ok := root[key]; ok {
			return v, true
		}
	}
	return nil, false
}

func absolutizePath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		rest := strings.TrimPrefix(p, "~")
		rest = strings.TrimPrefix(rest, "/")
		rest = strings.TrimPrefix(rest, `\`)
		p = filepath.Join(home, rest)
	}
	if !filepath.IsAbs(p) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(wd, p)
	}
	return filepath.Abs(p)
}

// IsProfileTree true, если команда принадлежит дереву profile.
func IsProfileTree(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "profile" {
			return true
		}
	}
	return false
}
