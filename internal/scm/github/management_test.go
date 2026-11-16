package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/scm"
	scmhttp "github.com/Snowbridge/git-herder/internal/scm/http"
)

type hit struct {
	method string
	path   string
	body   string
}

func newManagement(t *testing.T, h http.HandlerFunc) (*management, *[]hit) {
	t.Helper()
	var hits []hit
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		hits = append(hits, hit{method: r.Method, path: r.URL.EscapedPath(), body: string(b)})
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	c := NewRepositoryManagement(scmhttp.NewTransport(srv.Client(), "tok"), srv.URL)
	return c.(*management), &hits
}

func TestExistsStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		want   bool
		err    bool
	}{
		{http.StatusOK, true, false},
		{http.StatusNotFound, false, false},
		{http.StatusInternalServerError, false, true},
	}
	for _, tc := range cases {
		m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		})
		ok, err := m.Exists(context.Background(), "acme", "repo")
		if ok != tc.want || (err != nil) != tc.err {
			t.Fatalf("status %d ok %v err %v", tc.status, ok, err)
		}
		if tc.err {
			var he *scm.HTTPError
			if !errors.As(err, &he) || he.Status != tc.status {
				t.Fatalf("http error %v", err)
			}
		}
		if len(*hits) != 1 || (*hits)[0].method != http.MethodGet || (*hits)[0].path != "/repos/acme/repo" {
			t.Fatalf("hits %+v", *hits)
		}
	}
}

func TestContractBeforeHTTP(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	for _, call := range []struct{ path, name string }{
		{"", "repo"},
		{"acme", ""},
		{"acme/team", "repo"},
	} {
		if _, err := m.Exists(context.Background(), call.path, call.name); !errors.Is(err, scm.ErrContract) {
			t.Fatalf("exists %q %q: %v", call.path, call.name, err)
		}
		if err := m.Create(context.Background(), call.path, call.name); !errors.Is(err, scm.ErrContract) {
			t.Fatalf("create %q %q: %v", call.path, call.name, err)
		}
	}
	if len(*hits) != 0 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateOrg(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/orgs/acme":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/orgs/acme/repos":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	if err := m.Create(context.Background(), "acme", "repo"); err != nil {
		t.Fatal(err)
	}
	if len(*hits) != 2 {
		t.Fatalf("hits %+v", *hits)
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte((*hits)[1].body), &body); err != nil || body.Name != "repo" {
		t.Fatalf("body %s", (*hits)[1].body)
	}
	if strings.Contains((*hits)[1].body, "private") || strings.Contains((*hits)[1].body, "description") {
		t.Fatalf("extra fields %s", (*hits)[1].body)
	}
}

func TestCreateUser(t *testing.T) {
	t.Parallel()
	m, _ := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/me":
			w.WriteHeader(http.StatusNotFound)
		case "/users/me":
			w.WriteHeader(http.StatusOK)
		case "/user":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"me"}`))
		case "/user/repos":
			if r.Method != http.MethodPost {
				t.Errorf("method %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	if err := m.Create(context.Background(), "me", "repo"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateLoginMismatchSkipsPost(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/me":
			w.WriteHeader(http.StatusNotFound)
		case "/users/me":
			w.WriteHeader(http.StatusOK)
		case "/user":
			_, _ = w.Write([]byte(`{"login":"other"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	err := m.Create(context.Background(), "me", "repo")
	if !errors.Is(err, ErrLoginMismatch) {
		t.Fatalf("err %v", err)
	}
	for _, h := range *hits {
		if h.method == http.MethodPost {
			t.Fatalf("posted %+v", *hits)
		}
	}
}

func TestCreateBadUserJSONSkipsPost(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/me":
			w.WriteHeader(http.StatusNotFound)
		case "/users/me":
			w.WriteHeader(http.StatusOK)
		case "/user":
			_, _ = w.Write([]byte(`{`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})
	if err := m.Create(context.Background(), "me", "repo"); err == nil || errors.Is(err, ErrLoginMismatch) {
		t.Fatalf("err %v", err)
	}
	for _, h := range *hits {
		if h.method == http.MethodPost {
			t.Fatal("posted")
		}
	}
}

func TestCreateOrgErrorStops(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	err := m.Create(context.Background(), "acme", "repo")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 1 || (*hits)[0].path != "/orgs/acme" {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateOwnerMissing(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	err := m.Create(context.Background(), "ghost", "repo")
	if !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 2 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateUserErrorStops(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/me" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	err := m.Create(context.Background(), "me", "repo")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 2 || (*hits)[1].path != "/users/me" {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateNon201(t *testing.T) {
	t.Parallel()
	m, _ := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	err := m.Create(context.Background(), "acme", "repo")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusUnprocessableEntity {
		t.Fatalf("err %v", err)
	}
}

func TestReadPropertyContractBeforeHTTP(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	for _, call := range []struct{ owner, repo, property string }{
		{"", "repo", "workspace-path"},
		{"acme", "", "workspace-path"},
		{"acme/team", "repo", "workspace-path"},
		{"acme", "repo", ""},
	} {
		if _, err := m.ReadProperty(context.Background(), call.owner, call.repo, call.property); !errors.Is(err, scm.ErrContract) {
			t.Fatalf("read %q %q %q: %v", call.owner, call.repo, call.property, err)
		}
	}
	if len(*hits) != 0 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestReadPropertyValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		body  string
		state PropertyState
		value string
	}{
		{name: "string", body: `[{"property_name":"workspace-path","value":"domain\\billing"}]`, state: PropertyString, value: `domain\billing`},
		{name: "null", body: `[{"property_name":"workspace-path","value":null}]`, state: PropertyAbsent},
		{name: "missing", body: `[{"property_name":"other","value":"x"}]`, state: PropertyAbsent},
		{name: "empty", body: `[]`, state: PropertyAbsent},
		{name: "last", body: `[{"property_name":"workspace-path","value":"old"},{"property_name":"workspace-path","value":"domain/billing"}]`, state: PropertyString, value: "domain/billing"},
		{name: "number", body: `[{"property_name":"workspace-path","value":3}]`, state: PropertyNotString},
		{name: "bool", body: `[{"property_name":"workspace-path","value":true}]`, state: PropertyNotString},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			got, err := m.ReadProperty(context.Background(), "acme", "api", "workspace-path")
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.state || got.Value != tc.value {
				t.Fatalf("got %+v", got)
			}
			if len(*hits) != 1 || (*hits)[0].method != http.MethodGet || (*hits)[0].path != "/repos/acme/api/properties/values" {
				t.Fatalf("hits %+v", *hits)
			}
		})
	}
}

func TestReadPropertyHTTPError(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		m, _ := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})
		_, err := m.ReadProperty(context.Background(), "acme", "api", "workspace-path")
		var he *scm.HTTPError
		if !errors.As(err, &he) || he.Status != status {
			t.Fatalf("status %d err %v", status, err)
		}
	}
}

func TestPreflight(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/acme":
			w.WriteHeader(http.StatusOK)
		case "/orgs/acme/properties/schema/workspace-path":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"value_type":"string"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	if err := m.Preflight(context.Background(), "acme", "workspace-path"); err != nil {
		t.Fatal(err)
	}
	if len(*hits) != 2 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestPreflightOrgMissingDoesNotAskUser(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/acme" {
			t.Errorf("unexpected %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	})
	err := m.Preflight(context.Background(), "acme", "workspace-path")
	if !errors.Is(err, ErrOrgNotFound) {
		t.Fatal(err)
	}
	if len(*hits) != 1 {
		t.Fatalf("hits %+v", *hits)
	}
	if err := m.Preflight(context.Background(), "", "p"); !errors.Is(err, scm.ErrContract) {
		t.Fatal(err)
	}
	if err := m.Preflight(context.Background(), "acme/team", "p"); !errors.Is(err, scm.ErrContract) {
		t.Fatal(err)
	}
}

func TestPreflightProperty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusNotFound, ``, ErrPropertyNotFound},
		{http.StatusOK, `{"value_type":"single_select"}`, ErrPropertyNotString},
		{http.StatusInternalServerError, ``, nil},
	}
	for _, tc := range cases {
		m, _ := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/orgs/acme" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})
		err := m.Preflight(context.Background(), "acme", "tier")
		if tc.status == http.StatusInternalServerError {
			var he *scm.HTTPError
			if !errors.As(err, &he) || he.Status != tc.status {
				t.Fatalf("status %d err %v", tc.status, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d err %v", tc.status, err)
		}
	}
}

func TestWriteProperty(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/acme/api/properties/values" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := m.WriteProperty(context.Background(), "acme", "api", "workspace-path", "domain/billing/api"); err != nil {
		t.Fatal(err)
	}
	if len(*hits) != 1 || !strings.Contains((*hits)[0].body, `"property_name":"workspace-path"`) || !strings.Contains((*hits)[0].body, `"value":"domain/billing/api"`) {
		t.Fatalf("hits %+v", *hits)
	}
	m, _ = newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	err := m.WriteProperty(context.Background(), "acme", "api", "workspace-path", "x")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden {
		t.Fatal(err)
	}
	if err := m.WriteProperty(context.Background(), "acme", "", "p", "x"); !errors.Is(err, scm.ErrContract) {
		t.Fatal(err)
	}
}
