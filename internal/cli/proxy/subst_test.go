package proxy

import (
	"path/filepath"
	"testing"
)

func TestExpandTokenOnePass(t *testing.T) {
	t.Parallel()
	if got := expandToken("git@github.com:myorg/{path}.git", "api", "billing/api"); got != "git@github.com:myorg/billing/api.git" {
		t.Fatalf("%q", got)
	}
	if got := expandToken("{name}-{foo}-{path}", "api", "billing/api"); got != "api-{foo}-billing/api" {
		t.Fatalf("%q", got)
	}
	if got := expandToken("{name}{path}", "x", "a/{name}"); got != "xa/{name}" {
		t.Fatalf("reexpanded %q", got)
	}
	if got := expandToken("{pathname}", "n", "p"); got != "{pathname}" {
		t.Fatalf("%q", got)
	}
	if got := expandToken("pre{path}post", "n", "billing/api"); got != "prebilling/apipost" {
		t.Fatalf("%q", got)
	}
	if got := expandToken("origin", "api", "billing/api"); got != "origin" {
		t.Fatalf("%q", got)
	}
	src := []string{"add", "{name}"}
	out := expandTail(src, "api", "billing/api")
	if src[1] != "{name}" || out[1] != "api" {
		t.Fatalf("src=%q out=%q", src, out)
	}
}

func TestCopyMarkers(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "work")
	name, path, err := copyMarkers(root, filepath.Join(root, "billing", "api"))
	if err != nil || name != "api" || path != "billing/api" {
		t.Fatalf("name=%q path=%q err=%v", name, path, err)
	}
	name, path, err = copyMarkers(root, root)
	if err != nil || name != "work" || path != "" {
		t.Fatalf("root name=%q path=%q err=%v", name, path, err)
	}
}

func TestTailHasPath(t *testing.T) {
	t.Parallel()
	if !tailHasPath([]string{"add", "x/{path}.git"}) {
		t.Fatal("expected marker")
	}
	if tailHasPath([]string{"add", "{name}"}) {
		t.Fatal("name is not path")
	}
}
