// Пакет switchcmd реализует команду githerd switch.
package switchcmd

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
	"github.com/git-herder/git-herder/internal/redact"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/workspace"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду switch к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "switch <branch> [start-point]",
		Short: "Switch selected working copies to one branch",
		Long:  "Point each selected working copy at one local branch. No retry queue is used.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			return run(c, args, ui)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "Working-copy name criterion (repeatable)")
	fs.StringArray("qp", nil, "Working-copy path criterion (repeatable)")
	fs.BoolP("interactive", "i", false, "Choose working copies after the filters")
	fs.BoolP("all", "A", false, "Do not ask for confirmation above the mass threshold")
	fs.BoolP("create", "c", false, "Create the local branch")
	fs.BoolP("force-create", "C", false, "Create or recreate the local branch")
	fs.Bool("discard-changes", false, "Discard local changes and switch")
	fs.BoolP("merge", "m", false, "Carry local changes with a three-way merge")
	fs.String("track", "", "Remote for the upstream branch; bare --track means origin")
	fs.Lookup("track").NoOptDefVal = "origin"
	fs.StringArray("git-opts", nil, "One git switch argument per repeat, for example --git-opts=--ignore-other-worktrees")
	root.AddCommand(cmd)
}

var confirmMass = func(ui *console.UI, n int) (bool, error) {
	return ui.ConfirmMassPrompt(fmt.Sprintf("Continue switching %d working copies?", n))
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
	settings, err := cli.ResolveSwitch(cmd, args, rt.ProfileDoc)
	if err != nil {
		return fail(ui, err)
	}
	if err := validateSwitch(settings); err != nil {
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
		rt.Trace.Logger("cli.switch").Info(redact.String(text), "reason", "git")
	}
	resolver := workspace.Resolve().
		Workspace(g.Workspace).
		Path(settings.QP...).
		Name(settings.QN...).
		Interactive(settings.Interactive).
		Picker(asPicker(ui))
	if !settings.Create && !settings.ForceCreate {
		resolver = resolver.HasBranch(settings.Branch)
	}
	copies, err := resolver.Find(context.Background())
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
	steps := console.NewGitStep(os.Stderr, term.IsTerminal(int(os.Stderr.Fd())), len(jobs))
	bad := switchCopies(context.Background(), jobs, settings, runner, logf, steps, printLine, g.OnError == resolved.OnErrorAbort)
	if bad {
		return fail(ui, fmt.Errorf("one or more working copies were not switched"))
	}
	return nil
}

func validateSwitch(s cli.SwitchSettings) error {
	if s.Interactive && s.All {
		return fmt.Errorf("--interactive and --all cannot be used together")
	}
	if s.Branch == "" {
		return fmt.Errorf("branch is required")
	}
	if s.StartPointSet && s.StartPoint == "" {
		return fmt.Errorf("start-point is empty")
	}
	if s.StartPointSet && !s.Create && !s.ForceCreate {
		return fmt.Errorf("start-point requires --create or --force-create")
	}
	if s.Create && s.ForceCreate {
		return fmt.Errorf("--create and --force-create cannot be used together")
	}
	if s.DiscardChanges && s.Merge {
		return fmt.Errorf("--discard-changes and --merge cannot be used together")
	}
	if s.TrackSet && s.Track == "" {
		return fmt.Errorf("track remote is empty")
	}
	return cli.ValidateSwitchGitOpts(s.GitOpts)
}

func fail(ui *console.UI, err error) error {
	ui.Error(err.Error())
	return exit1(err)
}

func exit1(err error) error {
	return appexit.WithCode(1, err)
}
