package retrying

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

func TestExecuteSuccessRetryContinueAbort(t *testing.T) {
	t.Parallel()
	n, err := Execute([]int{1, 2}, 1, PolicyContinue, nil, func(v int) error {
		if v == 2 {
			return errors.New("boom")
		}
		return nil
	})
	if err != nil || n != 1 {
		t.Fatalf("continue n=%d err=%v", n, err)
	}

	calls := map[int]int{}
	n, err = Execute([]int{7}, 2, PolicyAbort, nil, func(v int) error {
		calls[v]++
		if calls[v] < 3 {
			return errors.New("temp")
		}
		return nil
	})
	if err != nil || n != 0 || calls[7] != 3 {
		t.Fatalf("retry n=%d calls=%d err=%v", n, calls[7], err)
	}

	_, err = Execute([]int{1}, 5, PolicyContinue, nil, func(int) error {
		return &AbortError{Err: errors.New("fatal")}
	})
	if err == nil || !errors.Is(err, errors.Unwrap(err)) && err.Error() != "fatal" {
		if err == nil || err.Error() != "fatal" {
			t.Fatalf("abort now %v", err)
		}
	}

	_, err = Execute([]int{1}, 0, PolicyAbort, nil, func(int) error { return errors.New("stop") })
	if err == nil {
		t.Fatal("expected abort")
	}
	_, err = Execute([]int{1}, -1, PolicyContinue, func(error) bool { return true }, func(int) error {
		return errors.New("nope")
	})
	if err == nil {
		t.Fatal("expected fatal")
	}
	if (&AbortError{}).Error() == "" {
		t.Fatal("empty abort")
	}
}

func TestExecuteDebugLogsDecision(t *testing.T) {
	tr, err := logging.New(logging.Options{Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	logging.Bind(tr)
	defer func() {
		logging.Unbind(tr)
		_ = tr.Close()
	}()

	if _, err := Execute([]int{1}, 1, PolicyContinue, nil, func(int) error {
		return errors.New("boom")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute([]int{1}, 0, PolicyAbort, nil, func(int) error {
		return errors.New("stop")
	}); err == nil {
		t.Fatal("expected abort")
	}
	body, err := os.ReadFile(tr.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"enter Execute",
		`"loggerName":"retrying"`,
		"decision=retry",
		"decision=continue",
		"decision=abort",
		"attempt=1",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}
