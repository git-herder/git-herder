// Пакет logging реализует Logger: NDJSON в файл и TSV на stderr при --verbose.
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	slogmulti "github.com/samber/slog-multi"

	"github.com/git-herder/git-herder/internal/redact"
	"github.com/git-herder/git-herder/internal/tstamp"
)

// Trace — фасад журнала выполнения на один запуск процесса.
type Trace struct {
	logger  *slog.Logger
	file    *os.File
	verbose bool
	debug   bool
	mu      sync.Mutex
}

// Options задаёт режим журнала.
type Options struct {
	Debug   bool
	Verbose bool
}

// New создаёт Trace, открывает файл журнала во временном каталоге.
func New(opts Options) (*Trace, error) {
	path, err := tstamp.ReserveLogFilePath(time.Now())
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	ndjson := &ndjsonHandler{w: f, debug: opts.Debug}
	handlers := []slog.Handler{ndjson}
	if opts.Verbose {
		handlers = append(handlers, &tsvHandler{w: os.Stderr, debug: opts.Debug})
	}
	l := slog.New(slogmulti.Fanout(handlers...))
	return &Trace{logger: l, file: f, verbose: opts.Verbose, debug: opts.Debug}, nil
}

// Logger возвращает slog.Logger с префиксом dot-notation.
func (t *Trace) Logger(name string) *slog.Logger {
	return t.logger.With("loggerName", name)
}

// Close закрывает файл журнала и снимает привязку, если она указывает на t.
func (t *Trace) Close() error {
	Unbind(t)
	if t.file == nil {
		return nil
	}
	return t.file.Close()
}

// FilePath — путь файла журнала. CLI пользователю его не показывает.
func (t *Trace) FilePath() string {
	if t == nil || t.file == nil {
		return ""
	}
	return t.file.Name()
}

// InfoStartCommand пишет стартовые записи INFO для команды.
func (t *Trace) InfoStartCommand(l *slog.Logger, profileLabel, cmdline string) {
	l.Info(redact.String("command start profile="+profileLabel),
		"reason", "start",
		"path", redact.CommandLine(cmdline))
	l.Info(redact.CommandLine(cmdline), "reason", "argv")
}

// WarnBrokenDefault — предупреждение о битом указателе default (английский текст).
func (t *Trace) WarnBrokenDefault(l *slog.Logger, name string) {
	msg := fmt.Sprintf("default profile pointer %q has no matching %s.yaml", name, name)
	l.Warn(redact.String(msg))
}

// LogError записывает ошибку: info без debug, error со stack при debug.
func (t *Trace) LogError(l *slog.Logger, err error) {
	if err == nil {
		return
	}
	msg := redact.String(err.Error())
	l.Info(msg, "reason", "error")
	if t.debug {
		l.Error(msg, "stackTrace", string(debug.Stack()))
	}
}

type ndjsonHandler struct {
	w     io.Writer
	debug bool
	attrs []slog.Attr
	mu    sync.Mutex
}

func (h *ndjsonHandler) Enabled(_ context.Context, level slog.Level) bool {
	if level == slog.LevelDebug {
		return h.debug
	}
	return level >= slog.LevelInfo
}

func (h *ndjsonHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	fields := map[string]any{
		"timestamp":  tstamp.DataTimestamp(r.Time),
		"level":      strings.ToLower(r.Level.String()),
		"loggerName": "git-herder",
		"message":    redact.String(r.Message),
	}
	if r.Level == slog.LevelError {
		for _, a := range attrsSlice(r) {
			if a.Key == "stackTrace" {
				fields["stackTrace"] = a.Value.String()
			}
		}
	}
	for _, a := range recordAttrs(h.attrs, r) {
		if a.Key == "loggerName" {
			fields["loggerName"] = a.Value.String()
		}
		if a.Key == "stackTrace" {
			fields["stackTrace"] = a.Value.String()
		}
	}
	if r.Level != slog.LevelError {
		delete(fields, "stackTrace")
	}
	b, err := marshalLog(fields)
	if err != nil {
		return err
	}
	_, err = h.w.Write(b)
	return err
}

// marshalLog пишет JSON без HTML-экранирования &, <, >.
func marshalLog(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *ndjsonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	dup := *h
	dup.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &dup
}

func (h *ndjsonHandler) WithGroup(string) slog.Handler { return h }

type tsvHandler struct {
	w     io.Writer
	debug bool
	attrs []slog.Attr
	mu    sync.Mutex
}

func (h *tsvHandler) Enabled(_ context.Context, level slog.Level) bool {
	if level == slog.LevelDebug {
		return h.debug
	}
	return level >= slog.LevelInfo
}

func (h *tsvHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	loggerName := "git-herder"
	for _, a := range recordAttrs(h.attrs, r) {
		if a.Key == "loggerName" {
			loggerName = a.Value.String()
		}
	}
	msg := escapeTSV(redact.String(truncate(r.Message)))
	line := strings.Join([]string{
		tstamp.DataTimestamp(r.Time),
		strings.ToLower(r.Level.String()),
		loggerName,
		"",
		msg,
	}, "\t")
	_, err := h.w.Write([]byte(line + "\n"))
	return err
}

func (h *tsvHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	dup := *h
	dup.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &dup
}

func (h *tsvHandler) WithGroup(string) slog.Handler { return h }

func escapeTSV(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

func attrsSlice(r slog.Record) []slog.Attr {
	var out []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		out = append(out, a)
		return true
	})
	return out
}

func recordAttrs(fixed []slog.Attr, r slog.Record) []slog.Attr {
	out := append([]slog.Attr{}, fixed...)
	out = append(out, attrsSlice(r)...)
	return out
}
