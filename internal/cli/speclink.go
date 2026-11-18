// Последняя строка справки: ссылка на спецификацию команды.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const specBase = "https://github.com/git-herder/documentation-specifications/blob/master/"

const specFileName = "index.md"

func attachSpecHelp(root *cobra.Command) {
	base := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		base(cmd, args)
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "\nFor more information refer to specification: "+formatSpecLine(specURL(cmd), writerIsTerminal(out)))
	})
}

func specURL(cmd *cobra.Command) string {
	if cmd == nil {
		return specBase + "cli/" + specFileName
	}
	name := cmd.Name()
	parent := ""
	if cmd.Parent() != nil {
		parent = cmd.Parent().Name()
	}
	if name == "profile" || parent == "profile" {
		return specBase + "cli/profile/" + specFileName
	}
	if parent == "" || name == "githerd" || name == "help" {
		return specBase + "cli/" + specFileName
	}
	return specBase + "cli/" + name + "/" + specFileName
}

func formatSpecLine(url string, terminal bool) string {
	if !terminal {
		return url
	}
	return "\033]8;;" + url + "\033\\" + specFileName + "\033]8;;\033\\"
}

func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
