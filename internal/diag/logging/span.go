package logging

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Snowbridge/git-herder/internal/redact"
	"github.com/Snowbridge/git-herder/internal/tstamp"
)

const maxValueBytes = 2048

var (
	current  atomic.Pointer[Trace]
	nopLeave = func(...any) {}
)

// Bind делает t журналом процесса. Пока привязки нет, Enter и Debug молчат.
func Bind(t *Trace) {
	if t == nil {
		return
	}
	current.Store(t)
}

// Unbind снимает привязку, только если сейчас привязан именно t.
func Unbind(t *Trace) {
	current.CompareAndSwap(t, nil)
}

// Enter пишет вход точки входа и возвращает функцию выхода.
// При выключенном debug аргументы не форматируются.
// Если журнал привяжут позже (старт команды), вход дописывается на выходе.
func Enter(kv ...any) func(...any) {
	tr := current.Load()
	if tr != nil && !tr.debug {
		return nopLeave
	}
	name, loggerName := callerName(2)
	entered := false
	if tr != nil && tr.debug {
		tr.debugLine(loggerName, "enter "+name+formatPairs(kv))
		entered = true
	}
	return func(results ...any) {
		tr := current.Load()
		if tr == nil || !tr.debug {
			return
		}
		if !entered {
			tr.debugLine(loggerName, "enter "+name+formatPairs(kv))
			entered = true
		}
		tr.debugLine(loggerName, "exit "+name+formatPairs(results))
	}
}

// Debug пишет диагностическую запись уровня debug от имени вызывающего пакета.
func Debug(message string, kv ...any) {
	tr := current.Load()
	if tr == nil || !tr.debug {
		return
	}
	_, loggerName := callerName(2)
	tr.debugLine(loggerName, message+formatPairs(kv))
}

func (t *Trace) debugLine(loggerName, message string) {
	if t == nil || t.logger == nil {
		return
	}
	t.logger.With("loggerName", loggerName).Debug(message)
}

func callerName(skip int) (fn, loggerName string) {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "git-herder"
	}
	f := runtime.FuncForPC(pc)
	if f == nil {
		return "unknown", "git-herder"
	}
	return splitCaller(f.Name())
}

func splitCaller(full string) (fn, loggerName string) {
	const prefix = "github.com/Snowbridge/git-herder/internal/"
	rest := strings.TrimPrefix(full, prefix)
	if i := strings.Index(rest, ")."); i >= 0 {
		head := rest[:i]
		method := trimGeneric(rest[i+2:])
		pkg := head
		typeName := ""
		if j := strings.Index(head, ".(*"); j >= 0 {
			pkg = head[:j]
			typeName = head[j+3:]
		}
		if typeName != "" {
			method = typeName + "." + method
		}
		return method, strings.ReplaceAll(pkg, "/", ".")
	}
	dot := strings.LastIndex(rest, ".")
	if dot < 0 {
		return trimGeneric(rest), "git-herder"
	}
	pkg := rest[:dot]
	fn = trimGeneric(rest[dot+1:])
	slash := strings.LastIndex(pkg, "/")
	tail := pkg
	if slash >= 0 {
		tail = pkg[slash+1:]
	}
	if i := strings.Index(tail, "."); i >= 0 {
		typeName := tail[i+1:]
		if slash < 0 {
			pkg = tail[:i]
		} else {
			pkg = pkg[:slash+1] + tail[:i]
		}
		fn = typeName + "." + fn
	}
	return fn, strings.ReplaceAll(pkg, "/", ".")
}

func trimGeneric(name string) string {
	if i := strings.Index(name, "["); i >= 0 {
		return name[:i]
	}
	return name
}

func formatPairs(kv []any) string {
	if len(kv) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(kv); {
		b.WriteByte(' ')
		if i+1 < len(kv) {
			key := fmt.Sprint(kv[i])
			b.WriteString(key)
			b.WriteByte('=')
			b.WriteString(formatValue(key, kv[i+1], 0))
			i += 2
			continue
		}
		b.WriteString(formatValue("", kv[i], 0))
		i++
	}
	return redact.String(b.String())
}

func formatValue(key string, v any, depth int) string {
	if redact.IsSecretKey(key) {
		return "***"
	}
	if v == nil {
		return "<nil>"
	}
	if _, ok := v.(context.Context); ok {
		return "ctx"
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		if rv.IsNil() {
			if rv.Kind() == reflect.Func {
				return "fn"
			}
			return "<nil>"
		}
	}
	if rv.Kind() == reflect.Func {
		return "fn"
	}
	switch x := v.(type) {
	case error:
		return truncate(x.Error())
	case string:
		return truncate(x)
	case []byte:
		return truncate(string(x))
	case time.Time:
		return truncate(tstamp.DataTimestamp(x))
	case time.Duration:
		return x.String()
	case http.Header:
		return truncate(formatHeader(x))
	case url.Values:
		return truncate(x.Encode())
	}
	if s, ok := formatNamedSlice(rv); ok {
		return truncate(s)
	}
	if rv.Kind() == reflect.Pointer && rv.Elem().Kind() != reflect.Struct {
		return formatValue(key, rv.Elem().Interface(), depth)
	}
	if depth < 2 && (rv.Kind() == reflect.Struct || (rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Struct)) {
		return truncate(formatStruct(rv, depth))
	}
	return truncate(fmt.Sprint(v))
}

func formatNamedSlice(rv reflect.Value) (string, bool) {
	if rv.Kind() != reflect.Slice {
		return "", false
	}
	elem := rv.Type().Elem()
	if elem.Kind() != reflect.Struct && elem.Kind() != reflect.Pointer {
		return "", false
	}
	m, ok := elem.MethodByName("FullName")
	if !ok || m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.String {
		return "", false
	}
	names := make([]string, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		if item.Kind() == reflect.Pointer && item.IsNil() {
			names[i] = "<nil>"
			continue
		}
		names[i] = item.MethodByName("FullName").Call(nil)[0].String()
	}
	return fmt.Sprintf("%d %s", rv.Len(), strings.Join(names, ",")), true
}

func formatStruct(rv reflect.Value, depth int) string {
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	t := rv.Type()
	var b strings.Builder
	b.WriteString(t.Name())
	b.WriteByte('{')
	first := true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(f.Name)
		b.WriteByte('=')
		fv := rv.Field(i)
		if !fv.CanInterface() {
			continue
		}
		if f.Type.Kind() == reflect.Func || (f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Func) {
			b.WriteString("fn")
			continue
		}
		b.WriteString(formatValue(f.Name, fv.Interface(), depth+1))
	}
	b.WriteByte('}')
	return b.String()
}

func formatHeader(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		val := strings.Join(h.Values(k), ",")
		if headerSecret(k) {
			val = "***"
		}
		parts = append(parts, k+"="+val)
	}
	return strings.Join(parts, ",")
}

func headerSecret(k string) bool {
	if redact.IsSecretKey(k) {
		return true
	}
	l := strings.ToLower(k)
	return strings.Contains(l, "cookie") || strings.Contains(l, "api-key") ||
		strings.Contains(l, "token") || strings.Contains(l, "password")
}

func truncate(s string) string {
	if len(s) <= maxValueBytes {
		return s
	}
	cut := maxValueBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + fmt.Sprintf("…(%d bytes)", len(s))
}
