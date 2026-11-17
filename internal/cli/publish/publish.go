// Пакет publish реализует команду githerd publish.
package publish

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
	"github.com/git-herder/git-herder/internal/redact"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/conn"
	"github.com/git-herder/git-herder/internal/scm/factory"
	gh "github.com/git-herder/git-herder/internal/scm/github"
	"github.com/git-herder/git-herder/internal/workspace"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду publish к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:   "publish [root]",
		Short: "Publish local working copies that are not yet on the SCM",
		Long:  "Create repositories for unpublished working copies, bind a remote, and push the default and current branches.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return run(c, args, ui)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("qp", nil, "Working-copy path criterion (repeatable)")
	fs.StringArray("qn", nil, "Working-copy name criterion (repeatable)")
	fs.String("path-attribute", "", "GitHub custom property: relative path of the working copy")
	fs.Bool("leaf-name", false, "On a flat SCM, use only the directory name as the repository name")
	fs.String("remote", "origin", "Remote name to bind")
	fs.String("existing", "skip", "Remote already points elsewhere: skip or rename")
	fs.BoolP("interactive", "i", false, "Choose working copies after the filters")
	fs.BoolP("all", "A", false, "Do not ask for confirmation above the mass threshold")
	fs.String("protocol", "ssh", "Remote URL protocol: ssh or https")
	root.AddCommand(cmd)
}

var openCreator = func(cfg conn.Config) (scm.RepositoryCreator, error) {
	return factory.NewClientFactory(nil).NewRepositoryCreator(cfg)
}

var confirmMass = func(ui *console.UI, n int) (bool, error) {
	return ui.ConfirmMass(n)
}

var printLine = func(s string) { fmt.Println(s) }

type preflightAPI interface {
	Preflight(ctx context.Context, org, property string) error
}

type githubPublishAPI interface {
	preflightAPI
	propAPI
}

func run(cmd *cobra.Command, args []string, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", cmd.Name())
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return exit1(fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolvePublish(cmd, args, rt.ProfileDoc)
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
	if settings.Interactive && settings.All {
		err := fmt.Errorf("--interactive and --all cannot be used together")
		ui.Error(err.Error())
		return exit1(err)
	}
	if strings.TrimSpace(settings.Root) == "" {
		err := fmt.Errorf("root is required")
		ui.Error(err.Error())
		return exit1(err)
	}
	segs, err := parseRoot(settings.Root)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	flat := flatSCM(g.Upstream)
	if flat && len(segs) != 1 {
		err := fmt.Errorf("flat SCM root must be one segment")
		ui.Error(err.Error())
		return exit1(err)
	}
	if !flat && len(segs) == 0 {
		err := fmt.Errorf("root is required")
		ui.Error(err.Error())
		return exit1(err)
	}
	if settings.LeafName && !flat {
		err := fmt.Errorf("--leaf-name is not allowed on a hierarchical SCM")
		ui.Error(err.Error())
		return exit1(err)
	}
	if err := cli.ValidatePathAttribute(settings.PathAttributeSet, settings.PathAttribute, g.Upstream); err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	host, err := gitHostOf(g.Host)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}

	cfg := conn.FromGlobals(g, 0, false)
	cfg.OnQuotaWait = func(until time.Time) {
		name := "GitHub"
		if g.Upstream == resolved.UpstreamGitLab {
			name = "GitLab"
		}
		ui.WarningStderr(name + " API rate limit reached; waiting until " + until.UTC().Format(time.RFC3339))
	}
	creator, err := openCreator(cfg)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	var writer propAPI
	if settings.PathAttributeSet {
		api, ok := creator.(githubPublishAPI)
		if !ok {
			err := fmt.Errorf("--path-attribute requires github")
			ui.Error(err.Error())
			return exit1(err)
		}
		if err := retryPreflight(context.Background(), api, segs[0], settings.PathAttribute, g.RetriesNumber); err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		writer = api
	}
	st, statErr := os.Stat(g.Workspace)
	if statErr != nil || !st.IsDir() {
		err := fmt.Errorf("workspace %s is not a directory", g.Workspace)
		ui.Error(err.Error())
		return exit1(err)
	}

	logf := func(text string) {
		if rt.Trace == nil || strings.TrimSpace(text) == "" {
			return
		}
		rt.Trace.Logger("cli.publish").Info(redact.String(text), "reason", "git")
	}
	copies, err := workspace.Resolve(workspace.Request{
		Root:        g.Workspace,
		QP:          settings.QP,
		QN:          settings.QN,
		Interactive: settings.Interactive,
		Picker:      ui,
		Keep: func(c workspace.Copy) (bool, error) {
			return keepByRemote(context.Background(), c.Dir, host, logf)
		},
	})
	if errors.Is(err, workspace.ErrCancelled) {
		return nil
	}
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if len(copies) == 0 {
		return nil
	}

	locs := make([]placed, len(copies))
	for i, c := range copies {
		loc, err := place(g.Workspace, c.Dir, segs, flat, settings.LeafName)
		if err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		locs[i] = loc
	}
	if flat {
		if err := duplicateName(locs); err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
	}

	var jobs []*publishJob
	for i, c := range copies {
		_, detached, out, err := runner.Head(context.Background(), c.Dir)
		logf(out)
		if err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		if detached {
			printLine(skipLine(locs[i].Rel))
			continue
		}
		addr, err := remoteAddress(host, fullPath(locs[i]), string(settings.Protocol), g.Upstream, g.GitHTTPSToken)
		if err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		jobs = append(jobs, &publishJob{
			dir:  c.Dir,
			rel:  locs[i].Rel,
			path: locs[i].Path,
			name: locs[i].Name,
			attr: locs[i].Attr,
			addr: addr,
		})
	}
	if len(jobs) > 10 && !settings.All && !settings.Interactive {
		ok, err := confirmMass(ui, len(jobs))
		if err != nil {
			ui.Error(err.Error())
			return exit1(err)
		}
		if !ok {
			return nil
		}
	}
	attrName := ""
	if settings.PathAttributeSet {
		attrName = settings.PathAttribute
	}
	steps := console.NewGitStep(os.Stderr, term.IsTerminal(int(os.Stderr.Fd())), len(jobs))
	failed, err := publishCopies(context.Background(), jobs, settings.Remote, settings.Existing, attrName, g.RetriesNumber, g.OnError, runner, creator, writer, logf, steps, printLine)
	if err != nil {
		ui.Error(err.Error())
		return exit1(err)
	}
	if failed {
		err := fmt.Errorf("one or more working copies were not published")
		ui.Error(err.Error())
		return exit1(err)
	}
	return nil
}

func keepByRemote(ctx context.Context, dir string, host gitHost, logf func(string)) (bool, error) {
	urls, out, err := runner.Remotes(ctx, dir)
	logf(out)
	if err != nil {
		return false, err
	}
	for _, raw := range urls {
		got, ok := remoteHost(raw)
		if ok && sameHost(got, host) {
			return false, nil
		}
	}
	return true, nil
}

func retryPreflight(ctx context.Context, api preflightAPI, org, property string, retries int) error {
	if retries < 0 {
		retries = 0
	}
	attempts := retries + 1
	var last error
	for i := 0; i < attempts; i++ {
		err := api.Preflight(ctx, org, property)
		if err == nil {
			return nil
		}
		if !preflightRetryable(err) {
			return err
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("github property preflight failed")
	}
	return last
}

func preflightRetryable(err error) bool {
	if errors.Is(err, gh.ErrOrgNotFound) || errors.Is(err, gh.ErrPropertyNotFound) || errors.Is(err, gh.ErrPropertyNotString) || errors.Is(err, scm.ErrContract) {
		return false
	}
	return true
}

func exit1(err error) error {
	return appexit.WithCode(1, err)
}
