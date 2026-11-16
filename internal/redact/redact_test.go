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

func TestCommandLineMasksGitHTTPSToken(t *testing.T) {
	in := "githerd --git-https-token=secret --git-https-token other"
	out := CommandLine(in)
	if strings.Contains(out, "secret") || strings.Contains(out, "other") {
		t.Fatalf("expected masked argv: %q", out)
	}
}

func TestStringMasksUserinfo(t *testing.T) {
	in := "push https://x-access-token:secret@github.com/acme/api.git"
	out := String(in)
	if strings.Contains(out, "secret") || strings.Contains(out, "x-access-token") {
		t.Fatalf("userinfo leaked: %q", out)
	}
	if !strings.Contains(out, "https://"+mask+"@github.com/acme/api.git") {
		t.Fatalf("url=%q", out)
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
	in["root"].(map[string]any)["git-https-token"] = "sek"
	out = MapYAML(in).(map[string]any)
	root = out["root"].(map[string]any)
	if root["git-https-token"] != mask {
		t.Fatalf("git https token not masked: %v", root["git-https-token"])
	}
}
