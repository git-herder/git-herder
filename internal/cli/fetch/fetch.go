// Команда fetch запускает git fetch в отобранных рабочих копиях.
// Разбор префикса и прогон общие с остальными прокси-командами.
package fetch

import (
	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/cli/proxy"
	"github.com/git-herder/git-herder/internal/diag/console"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду fetch к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:          "fetch [git arguments]",
		Short:        "Run git fetch in selected working copies",
		Long:         "Select working copies and run git fetch in each one. Git Herder flags stay in the prefix. {name} and {path} in the git tail expand for each copy.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return proxy.Run(c, args, "fetch", ui)
		},
	}
	proxy.AddFlags(cmd)
	root.AddCommand(cmd)
}
