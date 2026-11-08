package redact

import "testing"

func TestStringRedactAuthorization(t *testing.T) {
	in := "Authorization: Bearer xyz"
	out := String(in)
	if out == in {
		t.Fatal("expected redaction")
	}
}

func TestMapYAMLSlices(t *testing.T) {
	in := map[string]any{
		"items": []any{
			map[string]any{"token": "x"},
		},
	}
	out := MapYAML(in).(map[string]any)
	items := out["items"].([]any)
	m := items[0].(map[string]any)
	if m["token"] != mask {
		t.Fatal()
	}
}
