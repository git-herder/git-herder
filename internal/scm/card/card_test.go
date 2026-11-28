package card

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmptyProperties(t *testing.T) {
	t.Parallel()
	p := EmptyProperties()
	if len(p) != 0 {
		t.Fatal("expected empty map")
	}
}

func TestBranchesJSON(t *testing.T) {
	t.Parallel()
	c := Card{UID: TextUID("o/r"), Name: "r", URL: "https://ex/r"}
	c.Normalize()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, `"branches":[]`) {
		t.Fatalf("%s", text)
	}
	c.SetBranches([]string{"main"})
	b, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"branches":["main"]`) {
		t.Fatalf("%s", b)
	}
}
