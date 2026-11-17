package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
)

// ExistingPolicy — что делать с непустым каталогом назначения.
type ExistingPolicy string

const (
	ExistingSkip ExistingPolicy = "skip"
	ExistingDrop ExistingPolicy = "drop"
)

// Protocol — как собирать адрес для git clone.
type Protocol string

const (
	ProtocolSSH   Protocol = "ssh"
	ProtocolHTTPS Protocol = "https"
)

// CloneSettings — опции команды clone после резолва.
type CloneSettings struct {
	Criteria         scm.Criteria
	Existing         ExistingPolicy
	Protocol         Protocol
	PreservePath     bool
	Interactive      bool
	GitOpts          []string
	PathAttribute    string
	PathAttributeSet bool
}

// ResolveClone собирает опции clone и SCM-отбора из argv и профиля.
func ResolveClone(cmd *cobra.Command, doc map[string]any) (out CloneSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name())
	defer func() {
		leave(
			"existing", string(out.Existing),
			"protocol", string(out.Protocol),
			"preserve", out.PreservePath,
			"interactive", out.Interactive,
			"gitOpts", len(out.GitOpts),
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	out.Criteria, err = resolveCriteria(flags, cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	rawExisting, err := resolveStringFlag(flags, "existing", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Existing, err = parseExisting(rawExisting)
	if err != nil {
		return out, err
	}
	rawProtocol, err := resolveStringFlag(flags, "protocol", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Protocol, err = parseProtocol(rawProtocol)
	if err != nil {
		return out, err
	}
	out.PreservePath, err = resolveBoolFlag(flags, "preserve-path", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Interactive, err = resolveBoolFlag(flags, "interactive", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.GitOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-opts")
	if err != nil {
		return out, err
	}
	rawAttr, attrSet, err := resolveOptionalString(flags, cmd, doc, leaf, "path-attribute")
	if err != nil {
		return out, err
	}
	if attrSet {
		out.PathAttribute = strings.TrimSpace(rawAttr)
		out.PathAttributeSet = true
	}
	return out, nil
}

// ValidatePathAttribute проверяет флаг до запросов к SCM.
// Не задан — режим выключен. Пустое имя или upstream не github — ошибка.
func ValidatePathAttribute(set bool, name string, upstream resolved.Upstream) error {
	if !set {
		return nil
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--path-attribute is empty")
	}
	if upstream != resolved.UpstreamGitHub {
		return fmt.Errorf("--path-attribute requires github")
	}
	return nil
}

func parseExisting(raw string) (ExistingPolicy, error) {
	switch ExistingPolicy(strings.TrimSpace(raw)) {
	case "", ExistingSkip:
		return ExistingSkip, nil
	case ExistingDrop:
		return ExistingDrop, nil
	default:
		return "", fmt.Errorf("invalid --existing %q", raw)
	}
}

func parseProtocol(raw string) (Protocol, error) {
	switch Protocol(strings.TrimSpace(raw)) {
	case "", ProtocolSSH:
		return ProtocolSSH, nil
	case ProtocolHTTPS:
		return ProtocolHTTPS, nil
	default:
		return "", fmt.Errorf("invalid --protocol %q", raw)
	}
}

func resolveCriteria(flags *pflag.FlagSet, cmd *cobra.Command, doc map[string]any, leaf string) (scm.Criteria, error) {
	var out scm.Criteria
	out.Scope = resolved.SelectionScopeAffiliated

	paths, err := resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	names, err := resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	rawTags, err := resolveStringList(flags, cmd, doc, leaf, "qt")
	if err != nil {
		return out, err
	}
	rawAttrs, err := resolveStringList(flags, cmd, doc, leaf, "qa")
	if err != nil {
		return out, err
	}
	out.Paths = paths
	out.Names = names
	for _, raw := range rawTags {
		tag, err := scm.ParseTag(raw)
		if err != nil {
			return out, err
		}
		out.Tags = append(out.Tags, tag)
	}
	for _, raw := range rawAttrs {
		attr, err := scm.ParseAttr(raw)
		if err != nil {
			return out, err
		}
		out.Attrs = append(out.Attrs, attr)
	}

	searchRaw, searchSet, err := resolveSearch(flags, cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	if searchSet {
		pairs, err := scm.ParseSearch(searchRaw)
		if err != nil {
			return out, err
		}
		out.Search = true
		out.SearchPairs = pairs
	}

	scopeRaw, scopeSet, err := resolveOptionalString(flags, cmd, doc, leaf, "scope")
	if err != nil {
		return out, err
	}
	if scopeSet {
		scope, err := resolved.ParseSelectionScope(scopeRaw)
		if err != nil {
			return out, err
		}
		out.Scope = scope
	}

	if flags.Changed("include-inactive") {
		v, err := flags.GetBool("include-inactive")
		if err != nil {
			return out, err
		}
		out.IncludeInactive = v
		out.IncludeInactiveSet = true
	} else if v, ok := lookupProfileValue(doc, cmd, leaf, "include-inactive"); ok {
		b, err := asBool(v)
		if err != nil {
			return out, err
		}
		out.IncludeInactive = b
		out.IncludeInactiveSet = true
	}

	if flags.Changed("on-limit-exceeded") {
		v, err := flags.GetString("on-limit-exceeded")
		if err != nil {
			return out, err
		}
		out.OnLimit = strings.TrimSpace(v)
		out.OnLimitSet = true
	} else if v, ok := lookupProfileValue(doc, cmd, leaf, "on-limit-exceeded"); ok {
		out.OnLimit = strings.TrimSpace(fmt.Sprint(v))
		out.OnLimitSet = true
	}
	return out, nil
}

// ValidateGitOpts запрещает аргументы git, которые подменяют каталог, адрес или учётные данные.
func ValidateGitOpts(opts []string) error {
	for i := 0; i < len(opts); i++ {
		tok := opts[i]
		if tok == "--" || !strings.HasPrefix(tok, "-") {
			return fmt.Errorf("invalid --git-opts %q", tok)
		}
		if tok == "--separate-git-dir" || strings.HasPrefix(tok, "--separate-git-dir=") {
			return fmt.Errorf("invalid --git-opts %q", tok)
		}
		if tok == "-c" || tok == "--config" {
			if i+1 >= len(opts) {
				return fmt.Errorf("invalid --git-opts %q", tok)
			}
			i++
			if err := checkGitConfig(opts[i]); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "--config="); ok {
			if err := checkGitConfig(rest); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "-c="); ok {
			if err := checkGitConfig(rest); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkGitConfig(assignment string) error {
	key, _, ok := strings.Cut(assignment, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("invalid --git-opts config %q", assignment)
	}
	if forbiddenGitKey(key) {
		return fmt.Errorf("invalid --git-opts config %q", key)
	}
	return nil
}

func forbiddenGitKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(k, "credential.") {
		return true
	}
	if k == "http.extraheader" {
		return true
	}
	if keyed(k, "url.", ".insteadof") || keyed(k, "remote.", ".url") {
		return true
	}
	return false
}

func keyed(k, prefix, suffix string) bool {
	return strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) && len(k) > len(prefix)+len(suffix)
}
