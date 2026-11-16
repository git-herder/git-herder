package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// PublishExisting — что делать, если ремот уже указывает на другой URL.
type PublishExisting string

const (
	// PublishSkip не меняет копию и не отправляет ветки.
	PublishSkip PublishExisting = "skip"
	// PublishRename переименовывает текущий ремот и создаёт новый.
	PublishRename PublishExisting = "rename"
)

// PublishSettings — опции команды publish после резолва.
type PublishSettings struct {
	Root             string
	QN               []string
	QP               []string
	PathAttribute    string
	PathAttributeSet bool
	LeafName         bool
	Remote           string
	Existing         PublishExisting
	Interactive      bool
	All              bool
	Protocol         Protocol
}

// ResolvePublish собирает опции publish из argv и профиля.
// Ключ root читается только из аргумента или из блока publish.
func ResolvePublish(cmd *cobra.Command, args []string, doc map[string]any) (out PublishSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name(), "args", len(args))
	defer func() {
		leave(
			"root", out.Root,
			"remote", out.Remote,
			"existing", string(out.Existing),
			"protocol", string(out.Protocol),
			"leaf", out.LeafName,
			"interactive", out.Interactive,
			"all", out.All,
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	if len(args) > 0 {
		out.Root = strings.TrimSpace(args[0])
	} else if v, ok := publishBlockRoot(doc); ok {
		out.Root = v
	}
	out.QN, err = resolveStringList(flags, cmd, doc, leaf, "qn")
	if err != nil {
		return out, err
	}
	out.QP, err = resolveStringList(flags, cmd, doc, leaf, "qp")
	if err != nil {
		return out, err
	}
	rawAttr, attrSet, err := resolveOptionalString(flags, cmd, doc, leaf, "path-attribute")
	if err != nil {
		return out, err
	}
	if attrSet {
		out.PathAttribute = strings.TrimSpace(rawAttr)
		out.PathAttributeSet = true
	}
	out.LeafName, err = resolveBoolFlag(flags, "leaf-name", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	rawRemote, err := resolveStringFlag(flags, "remote", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Remote = strings.TrimSpace(rawRemote)
	if out.Remote == "" {
		out.Remote = "origin"
	}
	rawExisting, err := resolveStringFlag(flags, "existing", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Existing, err = parsePublishExisting(rawExisting)
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
	rawProtocol, err := resolveStringFlag(flags, "protocol", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Protocol, err = parseProtocol(rawProtocol)
	if err != nil {
		return out, err
	}
	return out, nil
}

func publishBlockRoot(doc map[string]any) (string, bool) {
	if doc == nil {
		return "", false
	}
	block, ok := doc["publish"].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := block["root"]
	if !ok {
		return "", false
	}
	return strings.TrimSpace(fmt.Sprint(v)), true
}

func parsePublishExisting(raw string) (PublishExisting, error) {
	switch PublishExisting(strings.TrimSpace(raw)) {
	case "", PublishSkip:
		return PublishSkip, nil
	case PublishRename:
		return PublishRename, nil
	default:
		return "", fmt.Errorf("invalid --existing %q", raw)
	}
}
