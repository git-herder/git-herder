package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestHandlersWithGroupAndDebug(t *testing.T) {
	var buf bytes.Buffer
	nd := &ndjsonHandler{w: &buf, debug: true}
	_ = nd.WithGroup("g")
	_ = nd.WithAttrs([]slog.Attr{slog.String("loggerName", "cli.test")})
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	_ = nd.Handle(context.Background(), rec)

	tsv := &tsvHandler{w: &buf, debug: false}
	_ = tsv.WithGroup("g")
	_ = tsv.Handle(context.Background(), rec)
}

func TestTSVTruncatesLongMessage(t *testing.T) {
	var buf bytes.Buffer
	h := &tsvHandler{w: &buf, debug: true}
	msg := strings.Repeat("h", 3000)
	rec := slog.NewRecord(time.Now(), slog.LevelDebug, msg, 0)
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "…(3000 bytes)") {
		t.Fatalf("suffix missing: %s", got)
	}
	if strings.Contains(got, strings.Repeat("h", 2049)) {
		t.Fatal("stderr message was not truncated")
	}
}
