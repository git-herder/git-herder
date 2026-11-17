package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// SwitchSettings — опции команды switch после резолва.
type SwitchSettings struct {
	Branch         string
	StartPoint     string
	StartPointSet  bool
	QN             []string
	QP             []string
	Interactive    bool
	All            bool
	Create         bool
	ForceCreate    bool
	DiscardChanges bool
	Merge          bool
	Track          string
	TrackSet       bool
	GitOpts        []string
}

// ResolveSwitch собирает опции switch.
// branch и start-point берутся из аргументов, иначе только из блока switch.
func ResolveSwitch(cmd *cobra.Command, args []string, doc map[string]any) (out SwitchSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name(), "args", len(args))
	defer func() {
		leave(
			"branch", out.Branch,
			"start", out.StartPointSet,
			"create", out.Create,
			"force", out.ForceCreate,
			"track", out.TrackSet,
			"opts", len(out.GitOpts),
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	if len(args) > 0 {
		out.Branch = strings.TrimSpace(args[0])
	} else if v, ok := leafString(doc, leaf, "branch"); ok {
		out.Branch = v
	}
	if len(args) > 1 {
		out.StartPoint = strings.TrimSpace(args[1])
		out.StartPointSet = true
	} else if v, ok := leafString(doc, leaf, "start-point"); ok {
		out.StartPoint = v
		out.StartPointSet = true
	}
	out.QN, err = resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	out.QP, err = resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	out.Interactive, err = resolveBoolFlag(flags, "interactive", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.All, err = resolveBoolFlag(flags, "all", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Create, err = resolveBoolFlag(flags, "create", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.ForceCreate, err = resolveBoolFlag(flags, "force-create", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.DiscardChanges, err = resolveBoolFlag(flags, "discard-changes", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Merge, err = resolveBoolFlag(flags, "merge", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	if flags.Changed("track") {
		out.Track, err = flags.GetString("track")
		if err != nil {
			return out, err
		}
		out.Track = strings.TrimSpace(out.Track)
		out.TrackSet = true
	} else if v, ok := lookupProfileValue(doc, cmd, leaf, "track"); ok {
		out.TrackSet = true
		if v == nil {
			out.Track = "origin"
		} else {
			out.Track = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	out.GitOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-opts")
	if err != nil {
		return out, err
	}
	return out, nil
}

// ValidateSwitchGitOpts запрещает токены clone и флаги, которые подменяют режим switch.
func ValidateSwitchGitOpts(opts []string) error {
	return validateGitArgList("git-opts", opts, switchOptBanned)
}

func switchOptBanned(tok string) bool {
	switch tok {
	case "-d", "-c", "-C", "-f", "-m", "-t":
		return true
	}
	if strings.HasPrefix(tok, "--force") {
		return true
	}
	for _, name := range []string{
		"--detach", "--orphan", "--guess", "--no-guess", "--create", "--force-create",
		"--discard-changes", "--merge", "--track",
	} {
		if tok == name || strings.HasPrefix(tok, name+"=") {
			return true
		}
	}
	return false
}

func leafValue(doc map[string]any, leaf, key string) (any, bool) {
	if doc == nil {
		return nil, false
	}
	block, ok := doc[leaf].(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := block[key]
	return v, ok
}

func leafString(doc map[string]any, leaf, key string) (string, bool) {
	v, ok := leafValue(doc, leaf, key)
	if !ok {
		return "", false
	}
	if v == nil {
		return "", true
	}
	return strings.TrimSpace(fmt.Sprint(v)), true
}

func validateGitArgList(flag string, opts []string, extra func(string) bool) error {
	for i := 0; i < len(opts); i++ {
		tok := opts[i]
		if tok == "--" || !strings.HasPrefix(tok, "-") {
			return fmt.Errorf("invalid --%s %q", flag, tok)
		}
		if tok == "--separate-git-dir" || strings.HasPrefix(tok, "--separate-git-dir=") {
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
			if err := checkGitArgConfig(flag, opts[i]); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "--config="); ok {
			if err := checkGitArgConfig(flag, rest); err != nil {
				return err
			}
			continue
		}
		if rest, ok := strings.CutPrefix(tok, "-c="); ok {
			if err := checkGitArgConfig(flag, rest); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkGitArgConfig(flag, assignment string) error {
	key, _, ok := strings.Cut(assignment, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("invalid --%s config %q", flag, assignment)
	}
	if forbiddenGitKey(key) {
		return fmt.Errorf("invalid --%s config %q", flag, key)
	}
	return nil
}
