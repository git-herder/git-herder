// Пакет report реализует команду githerd report.
package report

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/appexit"
	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/report"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/conn"
	"github.com/git-herder/git-herder/internal/scm/factory"
	"github.com/git-herder/git-herder/internal/scm/selection"
	"github.com/git-herder/git-herder/internal/tstamp"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду report к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "report",
		Short: "List SCM repositories to the console or a JSON file",
		Long:  "Select repositories on GitHub or GitLab and print them or write a JSON report. Does not modify SCM state.",
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
	fs.Bool("force", false, "Overwrite an existing JSON report file")
	fs.StringP("output", "o", "", "Write JSON report to this path; flag without a value uses a default file name")
	root.AddCommand(cmd)
}

// NormalizeArgs превращает отдельный --output / -o без значения в --output=
// (имя файла по умолчанию). Короткая форма -o= у pflag означает значение «=»,
// поэтому пустой -o тоже записывается длинной формой. --output путь не меняется.
func NormalizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out = append(out, args[i:]...)
			break
		}
		if a == "--output" || a == "-o" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				out = append(out, "--output=")
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func run(cmd *cobra.Command, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", cmd.Name())
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return exit1(fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolveReport(cmd, rt.ProfileDoc)
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
	if err := settings.Criteria.ValidateShape(g.Upstream); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if !settings.Criteria.HasCriterion() {
		ok, err := ui.ConfirmRiskyEnumeration()
		if err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		if !ok {
			return nil
		}
	}

	jsonPath, err := prepareOutput(settings)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	warnPublicScope(rt, ui, g.Host, settings.Criteria)

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
			rt.Trace.Logger("cli.report").Warn(msg)
		}
	}
	if jsonPath == "" {
		if err := report.WriteStdout(cards); err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		return nil
	}
	if err := report.WriteJSON(jsonPath, cards); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	ui.Success(jsonPath)
	return nil
}

func prepareOutput(settings cli.ReportSettings) (path string, err error) {
	leave := logging.Enter("stdout", settings.Stdout, "jsonPath", settings.JSONPath, "jsonDefault", settings.JSONDefault, "force", settings.Force)
	defer func() { leave("path", path, "err", err) }()
	if settings.Stdout {
		return "", nil
	}
	path = settings.JSONPath
	if settings.JSONDefault {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path, err = tstamp.ReserveReportPath(wd, time.Now())
		if err != nil {
			return "", err
		}
		return path, nil
	}
	if !settings.Force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("output file %s already exists (pass --force to overwrite)", path)
		}
	}
	return path, nil
}

func warnPublicScope(rt *cli.Runtime, ui *console.UI, host string, crit scm.Criteria) {
	if crit.Search || crit.Scope != resolved.SelectionScopeAll || !scm.PublicSaaSHost(host) {
		return
	}
	msg := "scope=all on a public SCM host disables the affiliation guard and may enumerate a very large catalog"
	ui.WarningStderr(msg)
	if rt.Trace != nil {
		rt.Trace.Logger("cli.report").Warn(msg)
	}
}

func exit1(err error) error {
	return appexit.WithCode(1, err)
}
