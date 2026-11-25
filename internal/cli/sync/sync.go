// Пакет sync реализует команду githerd sync.
package sync

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

// Register добавляет команду sync к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "sync [branch-name]",
		Short: "Pull and push one branch across working copies already on it",
		Long:  "Align the current branch of each selected working copy with the same branch on a remote.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return run(c, args, ui)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "Working-copy name criterion (repeatable)")
	fs.StringArray("qp", nil, "Working-copy path criterion (repeatable)")
	fs.String("remote", "origin", "Remote name for pull and push")
	fs.BoolP("interactive", "i", false, "Choose working copies after the filters")
	fs.BoolP("all", "A", false, "Do not ask for confirmation above the mass threshold")
	fs.StringArray("git-pull-opts", nil, "One git pull argument per repeat, for example --git-pull-opts=--ff-only")
	fs.StringArray("git-push-opts", nil, "One git push argument per repeat, for example --git-push-opts=--no-verify")
	root.AddCommand(cmd)
}

var confirmMass = func(ui *console.UI, n int) (bool, error) {
	return ui.ConfirmMassPrompt(fmt.Sprintf("Continue syncing %d working copies?", n))
}

var printLine = func(s string) { fmt.Println(s) }

var asPicker = func(ui *console.UI) workspace.Picker { return ui }

func run(cmd *cobra.Command, args []string, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", cmd.Name())
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return exit1(fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolveSync(cmd, args, rt.ProfileDoc)
	if err != nil {
		return fail(ui, err)
	}
	if settings.Interactive && settings.All {
		return fail(ui, fmt.Errorf("--interactive and --all cannot be used together"))
	}
	if settings.Branch == "" {
		return fail(ui, fmt.Errorf("branch is required"))
	}
	if settings.Remote == "" {
		return fail(ui, fmt.Errorf("remote name is empty"))
	}
	if err := cli.ValidateSyncGitOpts(settings.PullOpts, settings.PushOpts); err != nil {
		return fail(ui, err)
	}
	g := rt.Globals
	st, statErr := os.Stat(g.Workspace)
	if statErr != nil || !st.IsDir() {
		return fail(ui, fmt.Errorf("workspace %s is not a directory", g.Workspace))
	}
	logf := func(text string) {
		if rt.Trace == nil || strings.TrimSpace(text) == "" {
			return
		}
		rt.Trace.Logger("cli.sync").Info(redact.String(text), "reason", "git")
	}
	copies, err := workspace.Resolve().
		Workspace(g.Workspace).
		Path(settings.QP...).
		Name(settings.QN...).
		OnBranch(settings.Branch).
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
	jobs := make([]*syncJob, 0, len(copies))
	for _, c := range copies {
		rel, err := relToWorkspace(g.Workspace, c.Dir)
		if err != nil {
			return fail(ui, err)
		}
		jobs = append(jobs, &syncJob{dir: c.Dir, rel: rel})
	}
	auth := gitexec.HTTPSAuth{Token: g.GitHTTPSToken, Upstream: upstreamName(g.Upstream)}
	steps := console.NewGitStep(os.Stderr, term.IsTerminal(int(os.Stderr.Fd())), len(jobs))
	bad, err := syncCopies(context.Background(), jobs, settings.Branch, settings.Remote, settings.PullOpts, settings.PushOpts, auth, g.RetriesNumber, g.OnError, runner, logf, steps, printLine)
	if err != nil {
		return fail(ui, err)
	}
	if bad {
		return fail(ui, fmt.Errorf("one or more working copies were not synced"))
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
