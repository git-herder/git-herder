package cli

import (
	"os"
	"testing"
)

func TestNewRootCommand(t *testing.T) {
	root, err := NewRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	if root.Use != "githerd" {
		t.Fatalf("use=%q", root.Use)
	}
	if root.Version == "" {
		t.Fatal("missing version")
	}
}

func TestExitCodeFromError(t *testing.T) {
	if ExitCodeFromError(nil) != 0 {
		t.Fatal()
	}
	if ExitCodeFromError(errExit1(os.ErrInvalid)) != 1 {
		t.Fatal()
	}
}
