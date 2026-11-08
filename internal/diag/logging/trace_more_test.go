package logging

import "testing"

func TestTraceCloseNilFile(t *testing.T) {
	tr := &Trace{}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLogErrorNil(t *testing.T) {
	tr, _ := New(Options{})
	l := tr.Logger("t")
	tr.LogError(l, nil)
	_ = tr.Close()
}
