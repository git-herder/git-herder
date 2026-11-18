package cli_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/git-herder/git-herder/internal/bootstrap"
)

func TestHelpAppendsPlainSpecURL(t *testing.T) {
	t.Setenv("GITHERD_PROFILE_DIR", t.TempDir())
	cases := []struct {
		args []string
		url  string
	}{
		{[]string{"remote", "--help"}, "cli/remote/index.md"},
		{[]string{"profile", "create", "--help"}, "cli/profile/index.md"},
		{[]string{"--help"}, "cli/index.md"},
	}
	const base = "https://github.com/git-herder/documentation-specifications/blob/master/"
	for _, tc := range cases {
		out := captureStdout(t, func() {
			if code := bootstrap.RunWithArgs(tc.args); code != 0 {
				t.Fatalf("%v code=%d", tc.args, code)
			}
		})
		if !strings.HasSuffix(out, base+tc.url+"\n") {
			t.Fatalf("%v tail %q", tc.args, out)
		}
		if strings.Contains(out, "\033]8;;") {
			t.Fatalf("%v used a hyperlink", tc.args)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = prev
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
