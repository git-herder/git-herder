package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// MergeSettings — опции команды merge после резолва.
// ToSet ложно, когда [to] не задан и имя ещё нужно взять из git config.
type MergeSettings struct {
	From        string
	To          string
	ToSet       bool
	QN          []string
	QP          []string
	Remote      string
	Interactive bool
	All         bool
	Stay        bool
	KeepSource  bool
	NoFetch     bool
	GitOpts     []string
	FetchOpts   []string
	SwitchOpts  []string
	MergeOpts   []string
	BranchOpts  []string
}

// ResolveMerge собирает опции merge.
// from и to берутся из аргументов, иначе только из блока merge.
func ResolveMerge(cmd *cobra.Command, args []string, doc map[string]any) (out MergeSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name(), "args", len(args))
	defer func() {
		leave(
			"from", out.From,
			"toSet", out.ToSet,
			"remote", out.Remote,
			"stay", out.Stay,
			"keep", out.KeepSource,
			"noFetch", out.NoFetch,
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	if len(args) > 0 {
		out.From = strings.TrimSpace(args[0])
	} else if v, ok := leafString(doc, leaf, "from"); ok {
		out.From = v
	}
	if len(args) > 1 {
		out.To = strings.TrimSpace(args[1])
		out.ToSet = true
	} else if v, ok := leafString(doc, leaf, "to"); ok {
		out.To = v
		out.ToSet = true
	}
	out.QN, err = resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	out.QP, err = resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	if flags.Changed("remote") {
		out.Remote, err = flags.GetString("remote")
		if err != nil {
			return out, err
		}
		out.Remote = strings.TrimSpace(out.Remote)
	} else if v, ok := lookupProfileValue(doc, cmd, leaf, "remote"); ok {
		if v == nil {
			out.Remote = ""
		} else {
			out.Remote = strings.TrimSpace(fmt.Sprint(v))
		}
	} else {
		out.Remote, err = flags.GetString("remote")
		if err != nil {
			return out, err
		}
		out.Remote = strings.TrimSpace(out.Remote)
	}
	out.Interactive, err = resolveBoolFlag(flags, "interactive", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.All, err = resolveBoolFlag(flags, "all", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Stay, err = resolveBoolFlag(flags, "stay", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.KeepSource, err = resolveBoolFlag(flags, "keep-source-branch", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.NoFetch, err = resolveBoolFlag(flags, "no-fetch", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.GitOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-opts")
	if err != nil {
		return out, err
	}
	out.FetchOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-fetch-opts")
	if err != nil {
		return out, err
	}
	out.SwitchOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-switch-opts")
	if err != nil {
		return out, err
	}
	out.MergeOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-merge-opts")
	if err != nil {
		return out, err
	}
	out.BranchOpts, err = resolveStringList(flags, cmd, doc, leaf, "git-branch-opts")
	if err != nil {
		return out, err
	}
	return out, nil
}

// ValidateMergeGitOpts проверяет каждый список аргументов git команды merge.
func ValidateMergeGitOpts(gitOpts, fetchOpts, switchOpts, mergeOpts, branchOpts []string) error {
	lists := []struct {
		flag string
		opts []string
	}{
		{"git-opts", gitOpts},
		{"git-fetch-opts", fetchOpts},
		{"git-switch-opts", switchOpts},
		{"git-merge-opts", mergeOpts},
		{"git-branch-opts", branchOpts},
	}
	for _, list := range lists {
		if err := validateGitArgList(list.flag, list.opts, mergeOptBanned); err != nil {
			return err
		}
	}
	return nil
}

func mergeOptBanned(tok string) bool {
	for _, name := range []string{"--abort", "--continue", "--quit", "--edit"} {
		if tok == name || strings.HasPrefix(tok, name+"=") {
			return true
		}
	}
	return false
}
