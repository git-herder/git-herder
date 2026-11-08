package resolved

import "testing"

func TestBuiltinDefaults(t *testing.T) {
	t.Parallel()
	d := BuiltinDefaults()
	if d.OnError != OnErrorContinue || d.SelectionScope != SelectionScopeAffiliated {
		t.Fatalf("%+v", d)
	}
}

func TestParseUpstream(t *testing.T) {
	t.Parallel()
	u, err := ParseUpstream("")
	if err != nil || u != UpstreamUnset {
		t.Fatalf("%v %v", u, err)
	}
	u, err = ParseUpstream("GitHub")
	if err != nil || u != UpstreamGitHub {
		t.Fatalf("%v %v", u, err)
	}
	_, err = ParseUpstream("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultHost(t *testing.T) {
	t.Parallel()
	if DefaultHost(UpstreamUnset) != "" {
		t.Fatal("unset")
	}
	if DefaultHost(UpstreamGitHub) != "https://api.github.com" {
		t.Fatal(UpstreamGitHub)
	}
	if DefaultHost(UpstreamGitLab) != "https://gitlab.com/api/v4" {
		t.Fatal(UpstreamGitLab)
	}
}

func TestParseOnError(t *testing.T) {
	t.Parallel()
	p, err := ParseOnError("")
	if err != nil || p != OnErrorContinue {
		t.Fatalf("%v %v", p, err)
	}
	p, err = ParseOnError("abort")
	if err != nil || p != OnErrorAbort {
		t.Fatalf("%v %v", p, err)
	}
	_, err = ParseOnError("boom")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSelectionScope(t *testing.T) {
	t.Parallel()
	s, err := ParseSelectionScope("")
	if err != nil || s != SelectionScopeAffiliated {
		t.Fatalf("%v %v", s, err)
	}
	s, err = ParseSelectionScope("all")
	if err != nil || s != SelectionScopeAll {
		t.Fatalf("%v %v", s, err)
	}
	_, err = ParseSelectionScope("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}
