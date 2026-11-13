// Пакет gitexec: GitCliRunner запускает git из PATH.
package gitexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Snowbridge/git-herder/internal/diag/logging"
)

// Clone запускает `git clone` и возвращает объединённый stdout и stderr.
// Токен в аргументы и окружение не добавляется.
func Clone(ctx context.Context, opts []string, repoURL, dest string) (output string, err error) {
	leave := logging.Enter("opts", len(opts), "url", repoURL, "dest", dest)
	defer func() { leave("err", err) }()
	bin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH: %w", err)
	}
	return runBin(ctx, bin, cloneArgs(opts, repoURL, dest), os.Environ())
}

func cloneArgs(opts []string, repoURL, dest string) []string {
	args := make([]string, 0, 3+len(opts))
	args = append(args, "clone")
	args = append(args, opts...)
	args = append(args, "--", repoURL, dest)
	return args
}

func runBin(ctx context.Context, bin string, args, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	text := stdout.String() + stderr.String()
	if err != nil {
		return text, fmt.Errorf("git clone: %w", err)
	}
	return text, nil
}
