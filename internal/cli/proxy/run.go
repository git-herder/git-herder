// Общий прогон прокси-команд: отбор копий и один git на каждую без очереди повторов.
package proxy

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

// AddFlags вешает флаги префикса, общие для pull, push, fetch и remote.
func AddFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringArray("qn", nil, "Working-copy name criterion (repeatable)")
	fs.StringArray("qp", nil, "Working-copy path criterion (repeatable)")
	fs.BoolP("interactive", "i", false, "Choose working copies after the filters")
	fs.BoolP("all", "A", false, "Do not ask for confirmation above the mass threshold")
}

var confirmMass = func(ui *console.UI, verb string, n int) (bool, error) {
	return ui.ConfirmMassPrompt(fmt.Sprintf("Continue running git %s on %d working copies?", verb, n))
}

var printLine = func(s string) { fmt.Println(s) }

var asPicker = func(ui *console.UI) workspace.Picker { return ui }

// InstallIOForTest подменяет печать итога и вопрос порога.
func InstallIOForTest(print func(string), confirm func(*console.UI, string, int) (bool, error)) func() {
	prevP, prevC := printLine, confirmMass
	if print != nil {
		printLine = print
	}
	if confirm != nil {
		confirmMass = confirm
	}
	return func() {
		printLine = prevP
		confirmMass = prevC
	}
}

// Run выполняет прокси-команду с глаголом git verb.
func Run(cmd *cobra.Command, tail []string, verb string, ui *console.UI) (err error) {
	leave := logging.Enter("cmd", verb, "tail", len(tail))
	defer func() { leave("err", err) }()
	rt := cli.ActiveRuntime
	if rt == nil {
		return fail(ui, fmt.Errorf("CLI runtime is not initialized"))
	}
	settings, err := cli.ResolveProxy(cmd, rt.ProfileDoc)
	if err != nil {
		return fail(ui, err)
	}
	if settings.Interactive && settings.All {
		return fail(ui, fmt.Errorf("--interactive and --all cannot be used together"))
	}
	if settings.AllFromArgv && !hasToken(tail, "--all") {
		tokens := []string(nil)
		if call, ok := cli.ProxyCallFrom(cmd.Context()); ok {
			tokens = call.AllTokens
		}
		printLine(cli.FormatProxyNotice(tokens))
	}
	g := rt.Globals
	st, statErr := os.Stat(g.Workspace)
	if statErr != nil || !st.IsDir() {
		return fail(ui, fmt.Errorf("workspace %s is not a directory", g.Workspace))
	}
	copies, err := workspace.Resolve().
		Workspace(g.Workspace).
		Path(settings.QP...).
		Name(settings.QN...).
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
		ok, err := confirmMass(ui, verb, len(copies))
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
		name, path, err := copyMarkers(g.Workspace, c.Dir)
		if err != nil {
			return fail(ui, err)
		}
		jobs = append(jobs, copyJob{dir: c.Dir, rel: rel, name: name, path: path})
	}
	logf := func(text string) {
		if rt.Trace == nil || strings.TrimSpace(text) == "" {
			return
		}
		rt.Trace.Logger("cli.proxy").Info(redact.String(text), "reason", "git")
	}
	auth := gitexec.HTTPSAuth{Token: g.GitHTTPSToken, Upstream: upstreamName(g.Upstream)}
	steps := console.NewGitStep(os.Stderr, term.IsTerminal(int(os.Stderr.Fd())), len(jobs))
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	bad := runCopies(ctx, jobs, verb, tail, auth, runner, logf, steps, printLine, g.OnError == resolved.OnErrorAbort)
	if bad {
		return fail(ui, fmt.Errorf("one or more working copies failed"))
	}
	return nil
}

func hasToken(tail []string, tok string) bool {
	for _, item := range tail {
		if item == tok {
			return true
		}
	}
	return false
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
	if ui != nil {
		ui.Error(err.Error())
	}
	return appexit.WithCode(1, err)
}
