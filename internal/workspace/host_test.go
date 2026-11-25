package workspace

import "testing"

func TestHostOf(t *testing.T) {
	t.Parallel()
	got, err := HostOf("https://api.github.com")
	if err != nil || got.Name != "github.com" || got.Port != "" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = HostOf("https://gitlab.example:8443/api/v4")
	if err != nil || got.Name != "gitlab.example" || got.Port != "8443" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = HostOf("https://ghe.example/api/v3")
	if err != nil || got.Name != "ghe.example" || got.Port != "" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = HostOf("://"); err == nil {
		t.Fatal("expected invalid host")
	}
}

func TestHostOfRemote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		name string
		port string
		ok   bool
	}{
		{"git@github.com:acme/api.git", "github.com", "", true},
		{"https://github.com/acme/api.git", "github.com", "", true},
		{"ssh://git@gitlab.example:8443/g/api.git", "gitlab.example", "8443", true},
		{"https://x-access-token:secret@github.com/acme/api", "github.com", "", true},
		{"not a url", "", "", false},
	}
	for _, tc := range cases {
		got, ok := hostOfRemote(tc.raw)
		if ok != tc.ok || got.Name != tc.name || got.Port != tc.port {
			t.Fatalf("%q → %+v ok=%v", tc.raw, got, ok)
		}
	}
	api, _ := HostOf("https://api.github.com")
	gh, ok := hostOfRemote("git@github.com:acme/api.git")
	if !ok || !SameHost(api, gh) {
		t.Fatal("api.github.com must match github.com remotes")
	}
}

func TestSameHostCaseSensitive(t *testing.T) {
	if SameHost(Host{Name: "GitHub.com"}, Host{Name: "github.com"}) {
		t.Fatal("host comparison must be case-sensitive")
	}
	if !SameHost(Host{Name: "github.com", Port: "443"}, Host{Name: "github.com", Port: "443"}) {
		t.Fatal("equal hosts must match")
	}
}
