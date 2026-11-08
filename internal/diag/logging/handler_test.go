package logging

import (
	"bytes"
	"context"
	"log/slog"
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
