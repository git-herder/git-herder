package card

import (
	"encoding/json"
	"testing"
)

func TestUIDJSONAndSet(t *testing.T) {
	t.Parallel()
	n := NumberUID(42)
	b, err := json.Marshal(n)
	if err != nil || string(b) != "42" {
		t.Fatalf("%s %v", b, err)
	}
	var back UID
	if err := json.Unmarshal([]byte(`"org/repo"`), &back); err != nil || back.String() != "org/repo" {
		t.Fatal(err, back)
	}
	if err := json.Unmarshal([]byte(`7`), &back); err != nil || back.String() != "7" {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{}`), &back); err == nil {
		t.Fatal("expected error")
	}
	c := Card{Name: "spec", Path: "g/h", URL: "https://ex/s"}
	if c.FullName() != "g/h/spec" {
		t.Fatal(c.FullName())
	}
	c.Path = ""
	if c.FullName() != "spec" {
		t.Fatal(c.FullName())
	}
	var set Set
	set.Append(c)
	set.ReplaceLabels(0, nil)
	set.ReplaceProperties(0, map[string]any{"k": "v"})
	filtered := set.Filter(func(card Card) bool { return card.Properties["k"] == "v" })
	if len(filtered.Items) != 1 || len(filtered.Items[0].Labels) != 0 {
		t.Fatalf("%+v", filtered.Items)
	}
	c.Normalize()
	if c.Labels == nil || c.Properties == nil {
		t.Fatal("normalize")
	}
}

func TestCanonical(t *testing.T) {
	t.Parallel()
	if Canonical(true) != "true" || Canonical(false) != "false" {
		t.Fatal("bool")
	}
	if Canonical(json.Number("1.0")) != "1" || Canonical(json.Number("1.50")) != "1.50" {
		t.Fatal("number")
	}
	if Canonical("gold") != "gold" {
		t.Fatal("string")
	}
	if Canonical([]string{"a"}) != `["a"]` {
		t.Fatal(Canonical([]string{"a"}))
	}
	if _, ok := NormalizeNumber(""); ok {
		t.Fatal("empty")
	}
	if _, ok := NormalizeNumber("1e2"); ok {
		t.Fatal("exp")
	}
	if _, ok := NormalizeNumber("01"); ok {
		t.Fatal("leading zero")
	}
	if _, ok := NormalizeNumber("-"); ok {
		t.Fatal("minus")
	}
	if _, ok := NormalizeNumber(".5"); ok {
		t.Fatal("dot")
	}
	if n, ok := NormalizeNumber("-12"); !ok || n != "-12" {
		t.Fatal(n, ok)
	}
}
