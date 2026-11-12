package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/scm/card"
)

func TestWriteJSONSortsAndStdout(t *testing.T) {
	cards := []card.Card{
		{UID: card.TextUID("b"), Name: "b", Path: "z", URL: "https://ex/b"},
		{UID: card.NumberUID(1), Name: "a", URL: "https://ex/a"},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "r.json")
	if err := WriteJSON(path, cards); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `"uid":1`) || !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Fatalf("%s", text)
	}
	a := strings.Index(text, `"name":"a"`)
	b := strings.Index(text, `"name":"b"`)
	if a < 0 || b < 0 || a > b {
		t.Fatalf("sort %s", text)
	}
	if err := WriteJSON(filepath.Join(dir, "empty.json"), nil); err != nil {
		t.Fatal(err)
	}
	empty, _ := os.ReadFile(filepath.Join(dir, "empty.json"))
	if strings.TrimSpace(string(empty)) != "[]" {
		t.Fatalf("%s", empty)
	}
	if err := WriteStdout(cards[:1]); err != nil {
		t.Fatal(err)
	}
	plain := formatStdoutLine("g/a", "https://ex/a", false)
	if plain != "g/a\thttps://ex/a\n" {
		t.Fatalf("plain %q", plain)
	}
	linked := formatStdoutLine("g/a", "https://ex/a", true)
	if linked != "\033]8;;https://ex/a\033\\g/a\033]8;;\033\\\n" {
		t.Fatalf("link %q", linked)
	}
	if formatStdoutLine("g/a", "", true) != "g/a\t\n" {
		t.Fatal("empty url")
	}
}
