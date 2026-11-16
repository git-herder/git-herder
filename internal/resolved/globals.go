package resolved

// Globals — итог резолва глобальных флагов для текущей команды.
type Globals struct {
	Verbose        bool
	Debug          bool
	RetriesNumber  int
	OnError        OnError
	Workspace      string
	Upstream       Upstream
	Host           string // REST API base URL (как задал пользователь или DefaultHost)
	Token          string
	GitHTTPSToken  string
	ProfileName    string
	SelectionScope SelectionScope
}

// BuiltinDefaults встроенные значения из cli/index.md.
func BuiltinDefaults() Globals {
	return Globals{
		RetriesNumber:  3,
		OnError:        OnErrorContinue,
		SelectionScope: SelectionScopeAffiliated,
	}
}
