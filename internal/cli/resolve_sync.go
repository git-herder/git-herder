package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// SyncSettings — опции команды sync после резолва.
type SyncSettings struct {
	Branch      string
	QN          []string
	QP          []string
	Remote      string
	Interactive bool
	All         bool
	PullOpts    []string
	PushOpts    []string
}

// ResolveSync собирает опции sync. branch берётся из аргумента, иначе из блока sync.
func ResolveSync(cmd *cobra.Command, args []string, doc map[string]any) (out SyncSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name(), "args", len(args))
	defer func() {
		leave(
			"branch", out.Branch,
			"remote", out.Remote,
			"interactive", out.Interactive,
			"all", out.All,
			"pull", len(out.PullOpts),
			"push", len(out.PushOpts),
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	if len(args) > 0 {
		out.Branch = strings.TrimSpace(args[0])
	} else if v, ok := syncBlockString(doc, "branch"); ok {
		out.Branch = v
	}
	out.QN, err = resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	out.QP, err = resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	rawRemote, err := resolveStringFlag(flags, "remote", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Remote = strings.TrimSpace(rawRemote)
	out.Interactive, err = resolveBoolFlag(flags, "interactive", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.All, err = resolveBoolFlag(flags, "all", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.PullOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-pull-opts")
	if err != nil {
		return out, err
	}
	out.PushOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-push-opts")
	if err != nil {
		return out, err
	}
	return out, nil
}

func syncBlockString(doc map[string]any, key string) (string, bool) {
	if doc == nil {
		return "", false
	}
	block, ok := doc["sync"].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := block[key]
	if !ok {
		return "", false
	}
	return strings.TrimSpace(fmt.Sprint(v)), true
}

// ValidateSyncGitOpts запрещает токены и ключи конфигурации в списках sync.
// --separate-git-dir здесь не запрещён: его примет или отвергнет git.
func ValidateSyncGitOpts(pull, push []string) error {
	if err := validateSyncOptList("git-pull-opts", pull, pullOptBanned); err != nil {
		return err
	}
	return validateSyncOptList("git-push-opts", push, pushOptBanned)
}

func pullOptBanned(tok string) bool {
	return tok == "-r" || strings.HasPrefix(tok, "--rebase")
}

func pushOptBanned(tok string) bool {
	return tok == "-f" || strings.HasPrefix(tok, "--force")
}

func validateSyncOptList(flag string, opts []string, extra func(string) bool) error {
	for i := 0; i < len(opts); i++ {
		tok := opts[i]
		if tok == "--" || !strings.HasPrefix(tok, "-") {
			return fmt.Errorf("invalid --%s %q", flag, tok)
		}
		if extra != nil && extra(tok) {
			return fmt.Errorf("invalid --%s %q", flag, tok)
		}
		if tok == "-c" || tok == "--config" {
			if i+1 >= len(opts) {
				return fmt.Errorf("invalid --%s %q", flag, tok)
			}
			i++
			if err := checkSyncGitConfig(flag, opts[i]); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "--config="); ok {
			if err := checkSyncGitConfig(flag, rest); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "-c="); ok {
			if err := checkSyncGitConfig(flag, rest); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkSyncGitConfig(flag, assignment string) error {
	key, _, ok := strings.Cut(assignment, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("invalid --%s config %q", flag, assignment)
	}
	if forbiddenGitKey(key) {
		return fmt.Errorf("invalid --%s config %q", flag, key)
	}
	return nil
}
