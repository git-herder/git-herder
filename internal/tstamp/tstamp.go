// Пакет tstamp форматирует метки времени по ADR-26101704 (fs-timestamp, data-timestamp).
package tstamp

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DataTimestamp возвращает строку data-timestamp для полей журнала.
func DataTimestamp(t time.Time) string {
	loc := t.Location()
	y, m, d := t.Date()
	h, min, sec := t.Clock()
	ms := t.Nanosecond() / 1e6
	offSec := offsetSeconds(loc, t)
	sign := "+"
	if offSec < 0 {
		sign = "-"
		offSec = -offSec
	}
	oh := offSec / 3600
	om := (offSec % 3600) / 60
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03d%s%02d:%02d",
		y, int(m), d, h, min, sec, ms, sign, oh, om)
}

// FSTimestamp возвращает строку fs-timestamp для имён файлов.
func FSTimestamp(t time.Time) string {
	loc := t.Location()
	y, m, d := t.Date()
	h, min, sec := t.Clock()
	ms := t.Nanosecond() / 1e6
	offSec := offsetSeconds(loc, t)
	sign := "+"
	if offSec < 0 {
		sign = "-"
		offSec = -offSec
	}
	oh := offSec / 3600
	om := (offSec % 3600) / 60
	return fmt.Sprintf("%04d-%02d-%02dT%02d-%02d-%02d.%04d%s%02d%02d",
		y, int(m), d, h, min, sec, ms*10, sign, oh, om)
}

func offsetSeconds(loc *time.Location, t time.Time) int {
	_, off := t.In(loc).Zone()
	return off
}

// ReserveLogFilePath подбирает путь git-herder-<fs>.log во временном каталоге с учётом коллизий.
func ReserveLogFilePath(t time.Time) (string, error) {
	base := fmt.Sprintf("git-herder-%s.log", FSTimestamp(t))
	dir := os.TempDir()
	path := filepath.Join(dir, base)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	}
	for n := 2; n < 10000; n++ {
		stem := fmt.Sprintf("git-herder-%s-%d", FSTimestamp(t), n)
		candidate := filepath.Join(dir, stem+".log")
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tstamp: too many log file name collisions")
}
