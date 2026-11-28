// Пакет merge реализует команду githerd merge.
package merge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/git-herder/git-herder/internal/appexit"
	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/redact"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/workspace"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду merge к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "merge <from> [to]",
		Short: "Merge a local branch into another across working copies",
		Long:  "Fetch a remote, switch to the target branch, and merge a local branch. No retry queue is used.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			return run(c, args, ui)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "Working-copy name criterion (repeatable)")
	fs.StringArray("qp", nil, "Working-copy path criterion (repeatable)")
	fs.String("remote", "origin", "Remote fetched before the merge")
	fs.BoolP("interactive", "i", false, "Choose working copies after the filters")
	fs.BoolP("all", "A", false, "Do not ask for confirmation above the mass threshold")
	fs.BoolP("stay", "s", false, "Return to the entry branch after a successful merge")
	fs.BoolP("keep-source-branch", "k", false, "Keep the local source branch after a successful merge")
	fs.Bool("no-fetch", false, "Do not fetch the remote before the merge")
	fs.StringArray("git-opts", nil, "One git argument per repeat, added to every handler call")
	fs.StringArray("git-fetch-opts", nil, "One git fetch argument per repeat")
	fs.StringArray("git-switch-opts", nil, "One git switch argument per repeat")
	fs.StringArray("git-merge-opts", nil, "One git merge argument per repeat")
	fs.StringArray("git-branch-opts", nil, "One git branch argument per repeat")
	root.AddCommand(cmd)
}

var confirmMass = func(ui *console.UI, n int) (bool, error) {
	return ui.ConfirmMassPrompt(fmt.Sprintf("Continue merging %d working copies?", n))
}

var printLine = func(s string) { fmt.Println(s) }

var asPicker = func(ui *console.UI) workspace.Picker { return ui }

var newResolver = func() *workspace.Resolver { return workspace.Resolve() }

func run(cmd *cobra.Command, args []string, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", cmd.Name())
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return exit1(fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolveMerge(cmd, args, rt.ProfileDoc)
	if err != nil {
		return fail(ui, err)
	}
	if settings.Interactive && settings.All {
		return fail(ui, fmt.Errorf("--interactive and --all cannot be used together"))
	}
	if settings.From == "" {
		return fail(ui, fmt.Errorf("from is required"))
	}
	if settings.Remote == "" {
		return fail(ui, fmt.Errorf("remote name is empty"))
	}
	if settings.ToSet && settings.To == "" {
		return fail(ui, fmt.Errorf("to is empty"))
	}
	logf := func(text string) {
		if rt.Trace == nil || strings.TrimSpace(text) == "" {
			return
		}
		rt.Trace.Logger("cli.merge").Info(redact.String(text), "reason", "git")
	}
	if !settings.ToSet {
		name, out, derr := runner.DefaultBranch(context.Background())
		logOut(logf, out)
		if launchFailed(out, derr) {
			if derr == nil {
				derr = fmt.Errorf("git config failed")
			}
			return fail(ui, derr)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "master"
		}
		settings.To = name
	}
	if settings.From == settings.To {
		return fail(ui, fmt.Errorf("from and to are the same"))
	}
	if err := cli.ValidateMergeGitOpts(settings.GitOpts, settings.FetchOpts, settings.SwitchOpts, settings.MergeOpts, settings.BranchOpts); err != nil {
		return fail(ui, err)
	}
	g := rt.Globals
	st, statErr := os.Stat(g.Workspace)
	if statErr != nil || !st.IsDir() {
		return fail(ui, fmt.Errorf("workspace %s is not a directory", g.Workspace))
	}
	copies, err := newResolver().
		Workspace(g.Workspace).
		Path(settings.QP...).
		Name(settings.QN...).
		HasBranch(settings.From).
		Log(logf).
		Interactive(settings.Interactive).
		Picker(asPicker(ui)).
		Find(context.Background())
	if errors.Is(err, workspace.ErrCancelled) {
		return nil
	}
	if err != nil {
		return fail(ui, err)
	}
	if len(copies) == 0 {
		return nil
	}
	if len(copies) > 10 && !settings.All && !settings.Interactive {
		ok, err := confirmMass(ui, len(copies))
		if err != nil {
			return fail(ui, err)
		}
		if !ok {
			return nil
		}
	}
	jobs := make([]copyJob, 0, len(copies))
	for _, c := range copies {
		rel, err := relToWorkspace(g.Workspace, c.Dir)
		if err != nil {
			return fail(ui, err)
		}
		jobs = append(jobs, copyJob{dir: c.Dir, rel: rel})
	}
	auth := gitexec.HTTPSAuth{Token: g.GitHTTPSToken, Upstream: upstreamName(g.Upstream)}
	steps := console.NewGitStep(os.Stderr, term.IsTerminal(int(os.Stderr.Fd())), len(jobs))
	bad := mergeCopies(context.Background(), jobs, settings, auth, runner, logf, steps, printLine, g.OnError == resolved.OnErrorAbort)
	if bad {
		return fail(ui, fmt.Errorf("one or more working copies were not merged"))
	}
	return nil
}

func upstreamName(u resolved.Upstream) string {
	switch u {
	case resolved.UpstreamGitHub:
		return "github"
	case resolved.UpstreamGitLab:
		return "gitlab"
	default:
		return ""
	}
}

func fail(ui *console.UI, err error) error {
	ui.Error(err.Error())
	return exit1(err)
}

func exit1(err error) error {
	return appexit.WithCode(1, err)
}
