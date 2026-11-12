package console

import (
	"testing"
)

func TestUIHelpers(t *testing.T) {
	ui := New()
	ui.PrintLine("line")
	ui.PrintLines([]string{"a", "b"})
	ui.Error("err")
	ui.Warning("warn")
	ui.Success("ok")
	got := ui.HighlightDefault("name", true)
	if got == "" {
		t.Fatal("highlight empty")
	}
	got2 := ui.HighlightDefault("name", false)
	if got2 != "name" {
		t.Fatal(got2)
	}
	if _, err := ui.ConfirmRiskyEnumeration(); err == nil {
		t.Fatal("expected non-tty error")
	}
	ui.WarningStderr("warn")
	ok, err := ui.ConfirmDrop("x")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("non-tty should auto-confirm drop")
	}
}
