package version

import "testing"

func TestSemverNonEmpty(t *testing.T) {
	if Semver == "" {
		t.Fatal("semver must be set")
	}
}
