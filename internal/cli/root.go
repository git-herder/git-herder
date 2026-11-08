// Пакет cli: корень Cobra, глобальные флаги и хуки Invocation.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/appexit"
	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/profilestore"
	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/version"
	profilecmd "github.com/Snowbridge/git-herder/internal/cli/profile"
)

// Runtime — общее состояние запуска (Logger, Store).
type Runtime struct {
	Trace   *logging.Trace
	Store   *profilestore.Store
	Globals resolved.Globals
}

// ActiveRuntime устанавливается в PersistentPreRun для команд.
var ActiveRuntime *Runtime

// NewRootCommand собирает дерево githerd.
func NewRootCommand() (*cobra.Command, error) {
	store, err := profilestore.New()
	if err != nil {
		return nil, err
	}

	root := &cobra.Command{
		Use:   "githerd",
		Short: "Git Herder — mass operations on git repositories",
		Long:  "Git Herder (githerd) is a CLI for mass operations on git repositories.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if shouldSkipBootstrap(cmd) {
				return nil
			}
			return bootstrapRun(cmd, store)
		},
		SilenceUsage: true,
	}
	root.Version = version.Semver
	root.SetVersionTemplate("githerd version {{.Version}}\n")

	wd, _ := os.Getwd()
	pf := root.PersistentFlags()
	pf.BoolP("verbose", "V", false, "Mirror log events to stderr as TSV")
	pf.BoolP("debug", "D", false, "Verbose debug log in trace file")
	pf.IntP("retries-number", "R", 3, "Retries after first failure (0 = none)")
	pf.StringP("on-error", "e", "continue", "After retries: continue or abort")
	pf.StringP("workspace", "w", wd, "Workspace root directory (default: current directory)")
	pf.StringP("upstream", "U", "", "SCM platform: github or gitlab")
	pf.StringP("host", "H", "", "SCM host")
	pf.StringP("token", "t", "", "SCM API token (secret)")
	pf.String("profile", "", "Named YAML profile for this run")

	profilecmd.Register(root, store)

	return root, nil
}

func bootstrapRun(cmd *cobra.Command, store *profilestore.Store) error {
	flags := cmd.Root().PersistentFlags()

	if IsProfileTree(cmd) && flags.Changed("profile") {
		return errExit1(fmt.Errorf("the --profile flag is not allowed with the profile command"))
	}

	var doc map[string]any
	profileLabel := "none (profile command tree)"

	if IsProfileTree(cmd) {
		verbose, _ := flags.GetBool("verbose")
		debug, _ := flags.GetBool("debug")
		trace, err := logging.New(logging.Options{Debug: debug, Verbose: verbose})
		if err != nil {
			return err
		}
		globals := BuiltinDefaults()
		globals.Verbose = verbose
		globals.Debug = debug
		ActiveRuntime = &Runtime{Trace: trace, Store: store, Globals: globals}
		log := trace.Logger("cli.profile")
		trace.InfoStartCommand(log, profileLabel, strings.Join(os.Args, " "))
		return warnBrokenDefaultIfProfileTree(cmd, store, trace)
	}

	profileFlag, _ := flags.GetString("profile")
	name, loaded, err := store.LoadForResolve(profileFlag)
	if err != nil {
		return errExit1(err)
	}
	if loaded != nil {
		doc = loaded
		profileLabel = name
	} else {
		profileLabel = "none"
	}

	leaf := cmd.Name()
	if leaf == "" {
		leaf = "root"
	}
	globals, err := ResolveGlobals(cmd, doc, leaf)
	if err != nil {
		return errExit1(err)
	}
	trace, err := logging.New(logging.Options{Debug: globals.Debug, Verbose: globals.Verbose})
	if err != nil {
		return err
	}
	ActiveRuntime = &Runtime{Trace: trace, Store: store, Globals: globals}
	log := trace.Logger("cli.run")
	trace.InfoStartCommand(log, profileLabel, strings.Join(os.Args, " "))
	return nil
}

func warnBrokenDefaultIfProfileTree(cmd *cobra.Command, store *profilestore.Store, trace *logging.Trace) error {
	if !IsProfileTree(cmd) {
		return nil
	}
	broken, name, err := store.BrokenDefault()
	if err != nil {
		return err
	}
	if !broken {
		return nil
	}
	log := trace.Logger("cli.profile")
	trace.WarnBrokenDefault(log, name)
	msg := fmt.Sprintf("Warning: default profile pointer %q has no matching %s.yaml", name, name)
	fmt.Fprintln(os.Stderr, msg)
	return nil
}

func shouldSkipBootstrap(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.Version != "" && cmd.Flags().Changed("version") {
		return true
	}
	// help handled by cobra before PreRun on some paths — still init logger? spec says log every run
	if cmd.Flags().Changed("help") {
		return true
	}
	return false
}

func errExit1(err error) error {
	return ErrExitCode(1, err)
}

// ExitCodeFromError извлекает код выхода.
func ExitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var ec appexit.CodedError
	if errors.As(err, &ec) {
		return ec.Code
	}
	// Cobra usage / unknown subcommand
	if strings.Contains(err.Error(), "unknown command") ||
		strings.Contains(err.Error(), "required flag") ||
		strings.Contains(err.Error(), "accepts") {
		return 2
	}
	return 1
}
