package redact

import "testing"

func TestMapYAMLAnyKey(t *testing.T) {
	in := map[any]any{123: "plain", "token": "x"}
	out := MapYAML(in).(map[string]any)
	if out["token"] != mask {
		t.Fatal()
	}
}
