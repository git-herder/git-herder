package tstamp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDataTimestampFormat(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	tm := time.Date(2026, 8, 28, 12, 49, 0, 123000000, loc)
	got := DataTimestamp(tm)
	if !strings.Contains(got, "2026-08-28T12:49:00.123") {
		t.Fatalf("unexpected data timestamp: %q", got)
	}
	if !strings.Contains(got, "+03:00") {
		t.Fatalf("expected offset with colon: %q", got)
	}
}

func TestFSTimestampFormat(t *testing.T) {
	loc := time.FixedZone("MSK", 3*3600)
	tm := time.Date(2026, 8, 23, 21, 33, 56, 123000000, loc)
	got := FSTimestamp(tm)
	if strings.Contains(got, ":") {
		t.Fatalf("fs timestamp must not contain colon in time: %q", got)
	}
	if !strings.HasSuffix(got, "+0300") && !strings.Contains(got, "+0300") {
		t.Fatalf("expected fs offset: %q", got)
	}
}

func TestReserveLogFilePathCollision(t *testing.T) {
	tm := time.Now()
	path1, err := ReserveLogFilePath(tm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path1, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path2, err := ReserveLogFilePath(tm)
	if err != nil {
		t.Fatal(err)
	}
	if path1 == path2 {
		t.Fatalf("expected different paths on collision")
	}
	if !strings.Contains(filepath.Base(path2), "-2.log") {
		t.Fatalf("expected -2 suffix: %q", path2)
	}
	_ = os.Remove(path1)
	_ = os.Remove(path2)
}
