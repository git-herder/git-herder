// Операции обмена sync: pull, push, ls-remote и процессная подстановка https-токена.
package gitexec

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// Result — раздельные stdout и stderr одного запуска git.
type Result struct {
	Stdout string
	Stderr string
	Exit   int
}

// HTTPSAuth — секрет этого запуска. Пустой Token ничего не подставляет.
type HTTPSAuth struct {
	Token    string
	Upstream string
}

// InsteadOf собирает процессный аргумент
// git -c url.<https с userinfo>.insteadOf=<сохранённый URL>.
// Сохранённый URL в значение не переписывается.
func (a HTTPSAuth) InsteadOf(storedURL string) ([]string, error) {
	if strings.TrimSpace(a.Token) == "" {
		return nil, nil
	}
	stored := strings.TrimSpace(storedURL)
	with, err := a.httpsBase(stored)
	if err != nil {
		return nil, err
	}
	return []string{"-c", "url." + with + ".insteadOf=" + stored}, nil
}

func (a HTTPSAuth) user() string {
	if strings.EqualFold(strings.TrimSpace(a.Upstream), "gitlab") {
		return "oauth2"
	}
	return "x-access-token"
}

func (a HTTPSAuth) httpsBase(stored string) (string, error) {
	raw, err := toHTTPS(stored)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("remote url %q cannot take an https token", stored)
	}
	u.User = url.UserPassword(a.user(), a.Token)
	return u.String(), nil
}

func toHTTPS(stored string) (string, error) {
	s := strings.TrimSpace(stored)
	if s == "" {
		return "", fmt.Errorf("remote url %q cannot take an https token", stored)
	}
	if !strings.Contains(s, "://") {
		return scpToHTTPS(s)
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("remote url %q cannot take an https token", stored)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ssh":
		u.Scheme = "https"
		u.User = nil
		return u.String(), nil
	default:
		return "", fmt.Errorf("remote url %q cannot take an https token", stored)
	}
}

func scpToHTTPS(s string) (string, error) {
	at := strings.LastIndex(s, "@")
	colon := strings.LastIndex(s, ":")
	if at < 0 || colon < at+1 || colon >= len(s)-1 {
		return "", fmt.Errorf("remote url %q cannot take an https token", s)
	}
	host := s[at+1 : colon]
	path := strings.TrimPrefix(s[colon+1:], "/")
	if host == "" || path == "" || strings.Contains(host, "/") {
		return "", fmt.Errorf("remote url %q cannot take an https token", s)
	}
	return "https://" + host + "/" + path, nil
}

// RemoteNames возвращает имена из `git remote`.
func RemoteNames(ctx context.Context, dir string) (names []string, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("names", len(names), "err", err) }()
	res, err = runResult(ctx, dir, []string{"remote"})
	if err != nil {
		return nil, res, err
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, res, nil
}

// HeadOID возвращает идентификатор HEAD.
func HeadOID(ctx context.Context, dir string) (oid string, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "HEAD"})
	if err != nil {
		return "", res, err
	}
	return firstLine(res.Stdout), res, nil
}

// LsRemoteHead читает идентификатор refs/heads/<branch> на ремоте.
// cfg — процессные аргументы git до подкоманды, например -c url.*.insteadOf.
func LsRemoteHead(ctx context.Context, dir, remote, branch string, cfg []string) (oid string, found bool, res Result, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "branch", branch, "cfg", len(cfg))
	defer func() { leave("found", found, "exit", res.Exit, "err", err) }()
	args := append(append([]string{}, cfg...), "ls-remote", "--heads", remote, "refs/heads/"+branch)
	res, err = runResult(ctx, dir, args)
	if err != nil {
		return "", false, res, err
	}
	oid, found = parseRemoteOID(res.Stdout, "refs/heads/"+branch)
	return oid, found, res, nil
}

// Pull выполняет git pull --no-rebase --ff. opts стоят до `--`.
func Pull(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "branch", branch, "opts", len(opts), "cfg", len(cfg))
	defer func() { leave("exit", res.Exit, "err", err) }()
	args := append(append([]string{}, cfg...), "pull", "--no-rebase", "--ff")
	args = append(args, opts...)
	args = append(args, "--", remote, branch)
	return runResult(ctx, dir, args)
}

// PushOpts выполняет git push. opts стоят до `--`. Сигнатура Push без opts не меняется.
func PushOpts(ctx context.Context, dir string, opts []string, remote, branch string, cfg []string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "branch", branch, "opts", len(opts), "cfg", len(cfg))
	defer func() { leave("exit", res.Exit, "err", err) }()
	args := append(append([]string{}, cfg...), "push")
	args = append(args, opts...)
	args = append(args, "--", remote, branch)
	return runResult(ctx, dir, args)
}

// MergeHead сообщает, есть ли MERGE_HEAD. Код 1 — указателя нет.
func MergeHead(ctx context.Context, dir string) (ok bool, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("ok", ok, "exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "-q", "--verify", "MERGE_HEAD"})
	if err == nil {
		return true, res, nil
	}
	if res.Exit == 1 {
		return false, res, nil
	}
	return false, res, err
}

// Unmerged возвращает неслитые пути в порядке `git diff --name-only --diff-filter=U`.
func Unmerged(ctx context.Context, dir string) (names []string, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("names", len(names), "err", err) }()
	res, err = runResult(ctx, dir, []string{"diff", "--name-only", "--diff-filter=U"})
	if err != nil {
		return nil, res, err
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, res, nil
}

func parseRemoteOID(stdout, ref string) (string, bool) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		oid, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if strings.TrimSpace(name) == ref {
			return strings.TrimSpace(oid), true
		}
	}
	return "", false
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// Run запускает git с уже собранными аргументами.
// Каталог процесса — dir. Свой разделитель -- сюда не добавляется.
func Run(ctx context.Context, dir string, args []string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "n", len(args))
	defer func() { leave("exit", res.Exit, "err", err) }()
	return runResult(ctx, dir, args)
}

func runResult(ctx context.Context, dir string, args []string) (Result, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return Result{}, fmt.Errorf("git not found in PATH: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.Exit = exitErr.ExitCode()
	}
	return res, fmt.Errorf("git %s: %w", gitVerb(args), err)
}

func gitVerb(args []string) string {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if a == "-c" || a == "--config" {
			skip = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return "git"
}
