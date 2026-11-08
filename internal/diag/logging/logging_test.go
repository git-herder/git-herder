package logging

import (
	"os"
	"testing"
)

func TestNewAndClose(t *testing.T) {
	tr, err := New(Options{Debug: true, Verbose: true})
	if err != nil {
		t.Fatal(err)
	}
	l := tr.Logger("test.logger")
	l.Info("hello", "reason", "test")
	tr.LogError(l, os.ErrInvalid)
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewWithoutVerbose(t *testing.T) {
	tr, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	l := tr.Logger("cli.run")
	tr.InfoStartCommand(l, "none", "githerd --token=secret")
	tr.WarnBrokenDefault(l, "ghost")
	_ = tr.Close()
}
