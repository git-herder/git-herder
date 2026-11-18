// Команда remote запускает git remote в отобранных рабочих копиях.
// Подкоманды git, например add и set-url, остаются в хвосте.
package remote

import (
	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/cli/proxy"
	"github.com/git-herder/git-herder/internal/diag/console"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду remote к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:          "remote [git arguments]",
		Short:        "Run git remote in selected working copies",
		Long:         "Select working copies and run git remote in each one. Subcommands such as add and set-url are passed through to git.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return proxy.Run(c, args, "remote", ui)
		},
	}
	proxy.AddFlags(cmd)
	root.AddCommand(cmd)
}
