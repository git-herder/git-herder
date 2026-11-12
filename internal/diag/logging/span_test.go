package logging

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

type namedCard struct {
	name string
}

func (c namedCard) FullName() string { return c.name }

func TestEnterSilentWithoutDebug(t *testing.T) {
	leave := Enter("token", "sekret-value")
	leave("n", 1)

	tr, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	Bind(tr)
	defer func() {
		Unbind(tr)
		_ = tr.Close()
	}()
	leave = Enter("token", "sekret-value")
	leave("n", 1)
	body, err := os.ReadFile(tr.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "enter") || strings.Contains(string(body), "sekret-value") {
		t.Fatalf("debug off wrote %s", body)
	}
}

func TestEnterExitMasksAndTruncates(t *testing.T) {
	tr, err := New(Options{Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	Bind(tr)
	defer func() {
		Unbind(tr)
		_ = tr.Close()
	}()

	if err := traceSample("sekret-value"); err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer sekret-value")
	Debug("http request", "headers", h)
	Debug("http response", "body", strings.Repeat("x", 3000))
	Debug("cards", "cards", []namedCard{{"org/a"}, {"org/b"}})

	body, err := os.ReadFile(tr.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	s := logMessages(t, body)
	for _, want := range []string{
		"enter traceSample",
		"exit traceSample",
		"token=***",
		"err=<nil>",
		"Authorization=***",
		"…(3000 bytes)",
		"cards=2 org/a,org/b",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if !strings.Contains(string(body), `"loggerName":"diag.logging"`) {
		t.Fatalf("loggerName: %s", body)
	}
	if strings.Contains(s, "sekret-value") {
		t.Fatalf("secret leaked: %s", s)
	}
	if strings.Contains(s, strings.Repeat("x", 2049)) {
		t.Fatal("body was not truncated")
	}
}

func logMessages(t *testing.T, body []byte) string {
	t.Helper()
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var rec struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		b.WriteString(rec.Message)
		b.WriteByte('\n')
	}
	return b.String()
}

func traceSample(token string) (err error) {
	leave := Enter("token", token, "path", "a")
	defer func() { leave("err", err) }()
	return nil
}
