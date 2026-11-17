package main

import (
	"testing"

	"github.com/git-herder/git-herder/internal/bootstrap"
)

func TestMainDelegatesToBootstrap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHERD_PROFILE_DIR", dir)
	if code := bootstrap.RunWithArgs([]string{"--version"}); code != 0 {
		t.Fatalf("code=%d", code)
	}
}
