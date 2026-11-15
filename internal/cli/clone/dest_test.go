package clone

import (
	"path/filepath"
	"testing"
)

func TestDestinationOverlap(t *testing.T) {
	ws := filepath.Join("foo", "bar", "qwerty", "buzz", "fizz")
	got, err := destination(ws, "buzz/fizz/hrenator-figator-crud", false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ws, "hrenator-figator-crud")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestDestinationPreserve(t *testing.T) {
	ws := filepath.Join("foo", "bar", "qwerty", "buzz", "fizz")
	got, err := destination(ws, "buzz/fizz/hrenator-figator-crud", true)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ws, "buzz", "fizz", "hrenator-figator-crud")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestDestinationEmptyRemainder(t *testing.T) {
	ws := filepath.Join("code", "hrenator")
	if _, err := destination(ws, "hrenator", false); err == nil {
		t.Fatal("expected empty remainder")
	}
}

func TestAttributeDestination(t *testing.T) {
	ws := filepath.Join("work")
	got, err := attributeDestination(ws, "acme", "api", `domain\billing`, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, "domain", "billing", "api"); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	got, err = attributeDestination(ws, "acme", "api", `domain\billing`, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, "acme", "domain", "billing", "api"); got != want {
		t.Fatalf("preserve got %s want %s", got, want)
	}
	got, err = attributeDestination(ws, "acme", "api", ".", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, "api"); got != want {
		t.Fatalf("dot got %s want %s", got, want)
	}
	got, err = attributeDestination(ws, "acme", "api", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, "acme", "api"); got != want {
		t.Fatalf("dot preserve got %s want %s", got, want)
	}
	got, err = attributeDestination(ws, "acme", "api", `domain/./billing`, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(ws, "domain", "billing", "api"); got != want {
		t.Fatalf("slash got %s want %s", got, want)
	}
	if _, err := attributeDestination(ws, "acme", "api", `domain\..\billing`, false); err == nil {
		t.Fatal("expected .. error")
	}
}

func TestDestinationCaseSensitive(t *testing.T) {
	ws := filepath.Join("code", "Buzz")
	got, err := destination(ws, "buzz/name", false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ws, "buzz", "name")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRepoURL(t *testing.T) {
	got, err := repoURL("https://gitlab.example/g/api", "ssh")
	if err != nil || got != "git@gitlab.example:g/api.git" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = repoURL("https://gitlab.example/g/api.git", "ssh")
	if err != nil || got != "git@gitlab.example:g/api.git" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = repoURL("https://gitlab.example:2222/g/api", "ssh")
	if err != nil || got != "ssh://git@gitlab.example:2222/g/api.git" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = repoURL("https://gitlab.example/g/api", "https")
	if err != nil || got != "https://gitlab.example/g/api" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := repoURL("not a url", "ssh"); err == nil {
		t.Fatal("expected invalid url")
	}
}
