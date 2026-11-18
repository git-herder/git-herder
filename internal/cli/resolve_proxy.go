// Резолв флагов префикса прокси-команды: qn, qp, interactive и all.
package cli

import "github.com/spf13/cobra"

// ProxySettings — флаги отбора и порога после резолва с профилем.
type ProxySettings struct {
	QN          []string
	QP          []string
	Interactive bool
	All         bool
	// AllFromArgv истинно, когда argv включил all, а не только профиль.
	AllFromArgv bool
}

// ResolveProxy собирает ключи префикса текущей команды.
func ResolveProxy(cmd *cobra.Command, doc map[string]any) (out ProxySettings, err error) {
	flags := cmd.Flags()
	leaf := cmd.Name()
	out.QN, err = resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	out.QP, err = resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	out.Interactive, err = resolveBoolFlag(flags, "interactive", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.All, err = resolveBoolFlag(flags, "all", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	if flags.Changed("all") && out.All {
		out.AllFromArgv = true
	}
	return out, nil
}
