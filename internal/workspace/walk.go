// Обход каталога: папка .git задаёт рабочую копию и останавливает спуск.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// readDir читает каталог. В тесте подменяется, чтобы смоделировать ошибку чтения.
var readDir = os.ReadDir

func walk(root string) ([]Copy, error) {
	return walkDir(filepath.Clean(root))
}

func walkDir(dir string) ([]Copy, error) {
	entries, err := readDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.Name() == ".git" && e.IsDir() {
			return []Copy{{Dir: dir}}, nil
		}
	}
	var out []Copy
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := walkDir(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}
