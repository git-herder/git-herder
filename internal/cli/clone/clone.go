// Пакет clone реализует команду githerd clone.
package clone

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/git-herder/git-herder/internal/appexit"
	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/redact"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/conn"
	"github.com/git-herder/git-herder/internal/scm/factory"
	"github.com/git-herder/git-herder/internal/scm/github"
	"github.com/git-herder/git-herder/internal/scm/selection"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду clone к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone selected SCM repositories into the workspace",
		Long:  "Select repositories on GitHub or GitLab and clone them into --workspace.",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return run(c, ui)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("qp", nil, "Repository path criterion (repeatable)")
	fs.StringArray("qn", nil, "Repository name criterion (repeatable)")
	fs.StringArray("qt", nil, "Tag criterion (repeatable)")
	fs.StringArray("qa", nil, "Attribute criterion (repeatable)")
	fs.StringArray("scm-search", nil, "SCM-native search (single flag, space-separated key=value pairs)")
	fs.String("scope", "", "Enumeration scope: affiliated or all")
	fs.Bool("include-inactive", false, "GitLab: include inactive projects")
	fs.String("on-limit-exceeded", "", "GitHub rate limit policy: await or abort")
	fs.StringP("existing", "E", "skip", "Non-empty destination: skip or drop")
	fs.Bool("preserve-path", false, "Keep the repository path even when it repeats the workspace suffix")
	fs.BoolP("interactive", "i", false, "Choose repositories from the loaded list")
	fs.StringArray("git-opts", nil, "One git clone argument per repeat, for example --git-opts=--depth=1")
	fs.String("protocol", "ssh", "Clone URL protocol: ssh or https")
	fs.String("path-attribute", "", "GitHub custom property: parent directories of the clone inside the workspace")
	root.AddCommand(cmd)
}

var runGit gitFunc = func(ctx context.Context, opts []string, repoURL, dest string) (string, error) {
	return gitexec.Clone(ctx, opts, repoURL, dest)
}

func run(cmd *cobra.Command, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", cmd.Name())
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return exit1(fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolveClone(cmd, rt.ProfileDoc)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	g := rt.Globals
	if g.Upstream == resolved.UpstreamUnset {
		err := fmt.Errorf("upstream is required")
		ui.Error(err.Error())
		return exit1(err)
	}
	if strings.TrimSpace(g.Token) == "" {
		err := fmt.Errorf("token is required")
		ui.Error(err.Error())
		return exit1(err)
	}
	if err := cli.ValidatePathAttribute(settings.PathAttributeSet, settings.PathAttribute, g.Upstream); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if err := settings.Criteria.ValidateShape(g.Upstream); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if !settings.Criteria.HasCriterion() {
		err := fmt.Errorf("no selection criterion")
		ui.Error(err.Error())
		return exit1(err)
	}
	if err := cli.ValidateGitOpts(settings.GitOpts); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if settings.Interactive && !interactiveIO() {
		err := fmt.Errorf("interactive input is not available")
		ui.Error(err.Error())
		return exit1(err)
	}

	cfg := conn.FromGlobals(g, 0, settings.Criteria.IncludeInactive)
	cfg.SelectionScope = settings.Criteria.Scope
	cfg.OnLimit = settings.Criteria.OnLimit
	cfg.OnQuotaWait = func(until time.Time) {
		name := "GitHub"
		if g.Upstream == resolved.UpstreamGitLab {
			name = "GitLab"
		}
		ui.WarningStderr(name + " API rate limit reached; waiting until " + until.UTC().Format(time.RFC3339))
	}
	cards, err := selection.Run(context.Background(), factory.NewClientFactory(nil), cfg, settings.Criteria, selection.Options{
		Retries: g.RetriesNumber,
		OnError: g.OnError,
		Meter:   &selection.PtermMeter{},
	})
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if len(cards) == 0 {
		msg := "selection returned no repositories"
		ui.WarningStderr(msg)
		if rt.Trace != nil {
			rt.Trace.Logger("cli.clone").Warn(msg)
		}
		return nil
	}
	if settings.Interactive {
		cards, err = pick(ui, cards)
		if err != nil {
			ui.Error(err.Error())
			return err
		}
		if len(cards) == 0 {
			return nil
		}
	}
	if err := ensureWorkspace(g.Workspace); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	pol := existSkip
	if settings.Existing == cli.ExistingDrop {
		pol = existDrop
	}
	logf := func(text string) {
		if rt.Trace != nil {
			rt.Trace.Logger("cli.clone").Info(redact.String(text), "reason", "git")
		}
	}
	attrName := ""
	var props propertyReader
	if settings.PathAttributeSet {
		attrName = settings.PathAttribute
		creator, cerr := factory.NewClientFactory(nil).NewRepositoryCreator(cfg)
		if cerr != nil {
			ui.Error(cerr.Error())
			return exit1(cerr)
		}
		reader, ok := creator.(github.PropertyReader)
		if !ok {
			err := fmt.Errorf("--path-attribute requires github")
			ui.Error(err.Error())
			return exit1(err)
		}
		props = reader
	}
	failed, err := cloneCards(context.Background(), g.Workspace, cards, settings.PreservePath, string(settings.Protocol), pol, settings.GitOpts, g.RetriesNumber, g.OnError, runGit, logf, func(line string) {
		fmt.Println(line)
	}, attrName, props)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if failed {
		err := fmt.Errorf("one or more repositories were not cloned")
		ui.Error(err.Error())
		return exit1(err)
	}
	return nil
}

func pick(ui *console.UI, cards []card.Card) ([]card.Card, error) {
	items := make([]console.SelectItem, len(cards))
	for i, c := range cards {
		items[i] = console.SelectItem{Text: c.FullName(), URL: c.URL}
	}
	idx, err := ui.Select(items)
	if errors.Is(err, console.ErrSelectCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, exit1(err)
	}
	if len(idx) == 0 {
		return nil, nil
	}
	out := make([]card.Card, 0, len(idx))
	for _, i := range idx {
		out = append(out, cards[i])
	}
	return out, nil
}

func ensureWorkspace(path string) error {
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(path, 0o755)
	}
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("workspace %s is a file", path)
	}
	return nil
}

func interactiveIO() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func exit1(err error) error {
	return appexit.WithCode(1, err)
}
