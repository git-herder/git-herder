package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/scm"
)

// OutputEmpty — значение --output/--output= без пути (файл с именем по умолчанию).
const OutputEmpty = "\x00"

// ReportSettings — критерии отбора и режим вывода команды report.
type ReportSettings struct {
	Criteria    scm.Criteria
	Stdout      bool
	JSONPath    string
	JSONDefault bool
	Force       bool
}

// ResolveReport собирает опции report и SCM-отбора из argv и профиля.
func ResolveReport(cmd *cobra.Command, doc map[string]any) (out ReportSettings, err error) {
	leave := logging.Enter("leaf", cmd.Name())
	defer func() {
		leave(
			"stdout", out.Stdout,
			"jsonPath", out.JSONPath,
			"jsonDefault", out.JSONDefault,
			"force", out.Force,
			"search", out.Criteria.Search,
			"paths", out.Criteria.Paths,
			"names", out.Criteria.Names,
			"err", err,
		)
	}()
	flags := cmd.Flags()
	leaf := cmd.Name()
	out.Criteria, err = resolveCriteria(flags, cmd, doc, leaf)
	if err != nil {
		return out, err
	}

	force, err := resolveBoolFlag(flags, "force", cmd, doc, leaf)
	if err != nil {
		return out, err
	}
	out.Force = force

	if flags.Changed("output") {
		v, err := flags.GetString("output")
		if err != nil {
			return out, err
		}
		if v == "" || v == OutputEmpty {
			out.JSONDefault = true
			return out, nil
		}
		abs, err := absolutizePath(v)
		if err != nil {
			return out, err
		}
		out.JSONPath = abs
		return out, nil
	}
	if v, ok := lookupProfileValue(doc, cmd, leaf, "output"); ok {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			out.JSONDefault = true
			return out, nil
		}
		abs, err := absolutizePath(s)
		if err != nil {
			return out, err
		}
		out.JSONPath = abs
		return out, nil
	}
	out.Stdout = true
	return out, nil
}

func resolveStringList(flags interface {
	Changed(string) bool
	GetStringArray(string) ([]string, error)
}, cmd *cobra.Command, doc map[string]any, leaf, key string) ([]string, error) {
	if flags.Changed(key) {
		return flags.GetStringArray(key)
	}
	v, ok := lookupProfileValue(doc, cmd, leaf, key)
	if !ok {
		return nil, nil
	}
	return asStringList(v)
}

func resolveSearch(flags interface {
	Changed(string) bool
	GetStringArray(string) ([]string, error)
}, cmd *cobra.Command, doc map[string]any, leaf string) (string, bool, error) {
	if flags.Changed("scm-search") {
		vals, err := flags.GetStringArray("scm-search")
		if err != nil {
			return "", false, err
		}
		if len(vals) > 1 {
			return "", false, fmt.Errorf("scm-search flag repeated")
		}
		if len(vals) == 0 {
			return "", false, fmt.Errorf("invalid --scm-search")
		}
		return vals[0], true, nil
	}
	v, ok := lookupProfileValue(doc, cmd, leaf, "scm-search")
	if !ok {
		return "", false, nil
	}
	return strings.TrimSpace(fmt.Sprint(v)), true, nil
}

func resolveOptionalString(flags interface {
	Changed(string) bool
	GetString(string) (string, error)
}, cmd *cobra.Command, doc map[string]any, leaf, key string) (string, bool, error) {
	if flags.Changed(key) {
		v, err := flags.GetString(key)
		if err != nil {
			return "", false, err
		}
		return v, true, nil
	}
	v, ok := lookupProfileValue(doc, cmd, leaf, key)
	if !ok {
		return "", false, nil
	}
	return strings.TrimSpace(fmt.Sprint(v)), true, nil
}

func asStringList(v any) ([]string, error) {
	switch x := v.(type) {
	case string:
		return []string{x}, nil
	case []string:
		return append([]string(nil), x...), nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, fmt.Sprint(item))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid list value %T", v)
	}
}

func asBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, fmt.Errorf("invalid boolean %v", v)
}
