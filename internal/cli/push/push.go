// Команда push запускает git push в отобранных рабочих копиях.
// Разбор префикса и прогон общие с остальными прокси-командами.
package push

import (
	"github.com/spf13/cobra"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/cli/proxy"
	"github.com/git-herder/git-herder/internal/diag/console"
)

func init() {
	cli.AddCommand(Register)
}

// Register добавляет команду push к корню.
func Register(root *cobra.Command) {
	ui := console.New()
	cmd := &cobra.Command{
		Use:          "push [git arguments]",
		Short:        "Run git push in selected working copies",
		Long:         "Select working copies and run git push in each one. Git Herder flags stay in the prefix. The rest is passed to git unchanged.",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return proxy.Run(c, args, "push", ui)
		},
	}
	proxy.AddFlags(cmd)
	root.AddCommand(cmd)
}
