package redact

import (
	"strings"
	"testing"
)

func TestCommandLineMasksToken(t *testing.T) {
	in := "githerd --token=secret -t other"
	out := CommandLine(in)
	if strings.Contains(out, "secret") || strings.Contains(out, "other") {
		t.Fatalf("expected masked argv: %q", out)
	}
	if !strings.Contains(out, mask) {
		t.Fatal("expected *** in output")
	}
}

func TestMapYAMLMasksTokenKey(t *testing.T) {
	in := map[string]any{
		"root": map[string]any{"token": "abc", "host": "x"},
	}
	out := MapYAML(in).(map[string]any)
	root := out["root"].(map[string]any)
	if root["token"] != mask {
		t.Fatalf("token not masked: %v", root["token"])
	}
	if root["host"] != "x" {
		t.Fatal("host should remain")
	}
}
