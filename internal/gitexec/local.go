// Локальные операции switch и merge: чтение состояния и вызовы git без очереди повторов.
package gitexec

import (
	"context"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// SwitchRequest — аргументы одного git switch.
// Пустой StartPoint в вызов не входит.
type SwitchRequest struct {
	NoGuess     bool
	Discard     bool
	Merge       bool
	Create      bool
	ForceCreate bool
	Detach      bool
	Opts        []string
	Target      string
	StartPoint  string
}

// VerifyQuiet выполняет git rev-parse --verify --quiet <rev>.
// Код 0 — объект есть. Код 1 — объекта нет, это не ошибка. Иной код и сбой запуска — ошибка.
func VerifyQuiet(ctx context.Context, dir, rev string) (ok bool, res Result, err error) {
	leave := logging.Enter("dir", dir, "rev", rev)
	defer func() { leave("ok", ok, "exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "--verify", "--quiet", rev})
	if err == nil {
		return true, res, nil
	}
	if res.Exit == 1 {
		return false, res, nil
	}
	return false, res, err
}

// SymbolicHead читает символическую ветку HEAD.
// Код 0 — имя после refs/heads/. Иной код процесса — ветки нет, ошибки нет.
// Сбой запуска возвращает ошибку.
func SymbolicHead(ctx context.Context, dir string) (name string, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("name", name, "exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"symbolic-ref", "--quiet", "HEAD"})
	if err != nil && res.Exit == 0 {
		return "", res, err
	}
	if res.Exit != 0 {
		return "", res, nil
	}
	line := firstLine(res.Stdout)
	name = strings.TrimPrefix(line, "refs/heads/")
	return name, res, nil
}

// HeadCommitQuiet читает идентификатор HEAD.
// Код 0 — идентификатор. Код 1 — коммита нет. Иной код и сбой запуска — ошибка.
func HeadCommitQuiet(ctx context.Context, dir string) (oid string, missing bool, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("missing", missing, "exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "--verify", "--quiet", "HEAD"})
	if err == nil {
		return firstLine(res.Stdout), false, res, nil
	}
	if res.Exit == 1 {
		return "", true, res, nil
	}
	return "", false, res, err
}

// HeadCommitVerify читает идентификатор HEAD без --quiet.
// Ненулевой код и сбой запуска — ошибка: идентификатора нет.
func HeadCommitVerify(ctx context.Context, dir string) (oid string, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "--verify", "HEAD"})
	if err != nil {
		return "", res, err
	}
	return firstLine(res.Stdout), res, nil
}

// Porcelain сообщает, есть ли хотя бы одна строка git status --porcelain=v1.
func Porcelain(ctx context.Context, dir string) (dirty bool, res Result, err error) {
	leave := logging.Enter("dir", dir)
	defer func() { leave("dirty", dirty, "exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"status", "--porcelain=v1"})
	if err != nil {
		return false, res, err
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			return true, res, nil
		}
	}
	return false, res, nil
}

// GitPath возвращает путь git rev-parse --git-path.
func GitPath(ctx context.Context, dir, name string) (path string, res Result, err error) {
	leave := logging.Enter("dir", dir, "name", name)
	defer func() { leave("exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, dir, []string{"rev-parse", "--git-path", name})
	if err != nil {
		return "", res, err
	}
	return firstLine(res.Stdout), res, nil
}

// ConfigGet читает git config --get без -C рабочей копии.
// Код 1 — ключа нет: значение пустое, ошибка запуска сохраняется в err.
func ConfigGet(ctx context.Context, key string) (value string, res Result, err error) {
	leave := logging.Enter("key", key)
	defer func() { leave("exit", res.Exit, "err", err) }()
	res, err = runResult(ctx, "", []string{"config", "--get", key})
	return firstLine(res.Stdout), res, err
}

// BranchUpstream читает refs/heads/<branch>@{upstream}.
func BranchUpstream(ctx context.Context, dir, branch string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "branch", branch)
	defer func() { leave("exit", res.Exit, "err", err) }()
	ref := "refs/heads/" + branch + "@{upstream}"
	return runResult(ctx, dir, []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", ref})
}

// SetUpstream записывает вышестоящую ветку <remote>/<branch>.
func SetUpstream(ctx context.Context, dir, remote, branch string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "branch", branch)
	defer func() { leave("exit", res.Exit, "err", err) }()
	spec := "--set-upstream-to=" + remote + "/" + branch
	return runResult(ctx, dir, []string{"branch", spec, "--", branch})
}

// Fetch выполняет git fetch. cfg — процессные аргументы до подкоманды, opts — до --.
func Fetch(ctx context.Context, dir string, cfg, opts []string, remote string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "remote", remote, "cfg", len(cfg), "opts", len(opts))
	defer func() { leave("exit", res.Exit, "err", err) }()
	args := append(append([]string{}, cfg...), "fetch")
	args = append(args, opts...)
	args = append(args, "--", remote)
	return runResult(ctx, dir, args)
}

// Switch выполняет git switch по запросу.
func Switch(ctx context.Context, dir string, req SwitchRequest) (res Result, err error) {
	leave := logging.Enter("dir", dir, "target", req.Target, "opts", len(req.Opts))
	defer func() { leave("exit", res.Exit, "err", err) }()
	return runResult(ctx, dir, switchArgs(req))
}

func switchArgs(req SwitchRequest) []string {
	args := []string{"switch"}
	if req.NoGuess {
		args = append(args, "--no-guess")
	}
	if req.Discard {
		args = append(args, "--discard-changes")
	} else if req.Merge {
		args = append(args, "--merge")
	}
	creating := false
	if req.ForceCreate {
		args = append(args, "-C", req.Target)
		creating = true
	} else if req.Create {
		args = append(args, "-c", req.Target)
		creating = true
	}
	if req.Detach {
		args = append(args, "--detach")
	}
	args = append(args, req.Opts...)
	if creating {
		if req.StartPoint != "" {
			args = append(args, "--", req.StartPoint)
		}
		return args
	}
	args = append(args, "--", req.Target)
	if req.StartPoint != "" {
		args = append(args, req.StartPoint)
	}
	return args
}

// MergeBranch выполняет git merge --no-edit. opts стоят до --.
func MergeBranch(ctx context.Context, dir string, opts []string, from string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "from", from, "opts", len(opts))
	defer func() { leave("exit", res.Exit, "err", err) }()
	args := []string{"merge", "--no-edit"}
	args = append(args, opts...)
	args = append(args, "--", from)
	return runResult(ctx, dir, args)
}

// DeleteBranch выполняет git branch -d. opts стоят до --.
func DeleteBranch(ctx context.Context, dir string, opts []string, name string) (res Result, err error) {
	leave := logging.Enter("dir", dir, "name", name, "opts", len(opts))
	defer func() { leave("exit", res.Exit, "err", err) }()
	args := []string{"branch", "-d"}
	args = append(args, opts...)
	args = append(args, "--", name)
	return runResult(ctx, dir, args)
}
