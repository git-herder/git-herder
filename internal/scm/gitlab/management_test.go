package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestExistsEncodedPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		want   bool
		err    bool
	}{
		{http.StatusOK, true, false},
		{http.StatusNotFound, false, false},
		{http.StatusBadGateway, false, true},
	}
	for _, tc := range cases {
		m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		})
		ok, err := m.Exists(context.Background(), "group/sub", "proj")
		if ok != tc.want || (err != nil) != tc.err {
			t.Fatalf("status %d ok %v err %v", tc.status, ok, err)
		}
		if len(*hits) != 1 || (*hits)[0].path != "/api/v4/projects/group%2Fsub%2Fproj" {
			t.Fatalf("path %q", (*hits)[0].path)
		}
	}
}

func TestContractBeforeHTTP(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	for _, call := range []struct{ path, name string }{
		{"", "proj"},
		{"group", ""},
		{"group//sub", "proj"},
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

func TestCreateChain(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/a":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":10}`))
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/a%2Fb":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":20}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.EscapedPath())
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	if err := m.Create(context.Background(), "a/b", "proj"); err != nil {
		t.Fatal(err)
	}
	var group struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := json.Unmarshal([]byte((*hits)[2].body), &group); err != nil {
		t.Fatal(err)
	}
	if group.Name != "b" || group.Path != "b" || group.ParentID == nil || *group.ParentID != 10 {
		t.Fatalf("group %+v", group)
	}
	var project struct {
		Name        string         `json:"name"`
		Path        string         `json:"path"`
		NamespaceID int64          `json:"namespace_id"`
		Extra       map[string]any `json:"-"`
	}
	raw := map[string]any{}
	if err := json.Unmarshal([]byte((*hits)[3].body), &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte((*hits)[3].body), &project); err != nil {
		t.Fatal(err)
	}
	if project.Name != "proj" || project.Path != "proj" || project.NamespaceID != 20 || len(raw) != 3 {
		t.Fatalf("project %+v raw %v", project, raw)
	}
}

func TestCreateRootGroupOmitsParent(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/api/v4/groups":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":7}`))
		case r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	if err := m.Create(context.Background(), "root", "proj"); err != nil {
		t.Fatal(err)
	}
	var group map[string]any
	if err := json.Unmarshal([]byte((*hits)[1].body), &group); err != nil {
		t.Fatal(err)
	}
	if _, ok := group["parent_id"]; ok || group["name"] != "root" || group["path"] != "root" {
		t.Fatalf("group %v", group)
	}
	var project struct {
		NamespaceID int64 `json:"namespace_id"`
	}
	if err := json.Unmarshal([]byte((*hits)[2].body), &project); err != nil || project.NamespaceID != 7 {
		t.Fatalf("project %s err %v", (*hits)[2].body, err)
	}
}

func TestCreateGroupErrorStopsChain(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() == "/api/v4/groups/a" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	err := m.Create(context.Background(), "a/b", "proj")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 2 {
		t.Fatalf("hits %+v", *hits)
	}
	for _, h := range *hits {
		if h.method == http.MethodPost {
			t.Fatal("posted")
		}
	}
}

func TestCreateMissingGroupID(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
	})
	if err := m.Create(context.Background(), "a", "proj"); !errors.Is(err, errMissingGroupID) {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 1 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateGroup201MissingID(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/v4/groups" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
	})
	if err := m.Create(context.Background(), "a", "proj"); !errors.Is(err, errMissingGroupID) {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 2 {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateGroupPostNon201(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})
	err := m.Create(context.Background(), "a", "proj")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusBadRequest {
		t.Fatalf("err %v", err)
	}
	if len(*hits) != 2 || (*hits)[1].path != "/api/v4/groups" {
		t.Fatalf("hits %+v", *hits)
	}
}

func TestCreateProjectNon201(t *testing.T) {
	t.Parallel()
	m, _ := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":3}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
	})
	err := m.Create(context.Background(), "a", "proj")
	var he *scm.HTTPError
	if !errors.As(err, &he) || he.Status != http.StatusConflict {
		t.Fatalf("err %v", err)
	}
}

func TestCreateDoesNotCheckProject(t *testing.T) {
	t.Parallel()
	m, hits := newManagement(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/a" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":3}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	if err := m.Create(context.Background(), "a", "proj"); err != nil {
		t.Fatal(err)
	}
	for _, h := range *hits {
		if h.path != "/api/v4/groups/a" && h.path != "/api/v4/projects" {
			t.Fatalf("path %s", h.path)
		}
	}
}
