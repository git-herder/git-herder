// Пакет gitexec: GitCliRunner запускает git из PATH.
package gitexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
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

// Remotes возвращает URL всех ремотов копии, и fetch, и push.
func Remotes(ctx context.Context, dir string) (urls []string, output string, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("urls", len(urls), "err", err) }()
	output, err = runIn(ctx, dir, []string{"remote", "-v"})
	if err != nil {
		return nil, output, err
	}
	return parseRemoteVerbose(output), output, nil
}

// HeadBranch — текущая ветка. detached истинно, когда HEAD не указывает на ветку.
func HeadBranch(ctx context.Context, dir string) (branch string, detached bool, output string, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("branch", branch, "detached", detached, "err", err) }()
	output, err = runIn(ctx, dir, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if err != nil {
		return "", false, output, err
	}
	branch = strings.TrimSpace(output)
	if i := strings.IndexAny(branch, "\r\n"); i >= 0 {
		branch = strings.TrimSpace(branch[:i])
	}
	if branch == "" || branch == "HEAD" {
		return "", true, output, nil
	}
	return branch, false, output, nil
}

// LocalBranch сообщает, есть ли локальная ветка refs/heads/<name>.
// Код выхода 1 — ветки нет. Прочий код — ошибка.
func LocalBranch(ctx context.Context, dir, name string) (ok bool, output string, err error) {
	leave := logging.Enter("dir", dir, "name", name)
	defer func() { leave("ok", ok, "err", err) }()
	output, err = runIn(ctx, dir, []string{"show-ref", "--verify", "--quiet", "refs/heads/" + name})
	if err == nil {
		return true, output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, output, nil
	}
	return false, output, err
}

// RemoteURL читает URL ремота. missing — ремота с таким именем нет.
func RemoteURL(ctx context.Context, dir, name string) (remoteURL string, missing bool, output string, err error) {
	leave := logging.Enter("dir", dir, "name", name)
	defer func() { leave("missing", missing, "err", err) }()
	output, err = runIn(ctx, dir, []string{"remote", "get-url", name})
	if err == nil {
		return strings.TrimSpace(output), false, output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && (exitErr.ExitCode() == 2 || strings.Contains(output, "No such remote")) {
		return "", true, output, nil
	}
	return "", false, output, err
}

// RemoteAdd создаёт ремот с заданным URL.
func RemoteAdd(ctx context.Context, dir, name, remoteURL string) (output string, err error) {
	leave := logging.Enter("dir", dir, "name", name, "url", remoteURL)
	defer func() { leave("err", err) }()
	return runIn(ctx, dir, []string{"remote", "add", name, remoteURL})
}

// RemoteRename переименовывает ремот.
func RemoteRename(ctx context.Context, dir, oldName, newName string) (output string, err error) {
	leave := logging.Enter("dir", dir, "from", oldName, "to", newName)
	defer func() { leave("err", err) }()
	return runIn(ctx, dir, []string{"remote", "rename", oldName, newName})
}

// Push отправляет одну локальную ветку в ремот. Токен в аргументы и окружение не добавляется.
func Push(ctx context.Context, dir, remote, branch string) (output string, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "branch", branch)
	defer func() { leave("err", err) }()
	return runIn(ctx, dir, []string{"push", remote, branch})
}

func parseRemoteVerbose(out string) []string {
	var urls []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.LastIndex(line, " ("); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		urls = append(urls, fields[len(fields)-1])
	}
	return urls
}

func runIn(ctx context.Context, dir string, args []string) (string, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	text := stdout.String() + stderr.String()
	if err != nil {
		verb := "git"
		if len(args) > 0 {
			verb = "git " + args[0]
		}
		return text, fmt.Errorf("%s: %w", verb, err)
	}
	return text, nil
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
