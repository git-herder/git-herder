// Реализация критериев отбора: проверка одной копии через git.
package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/gitexec"
)

// keepHasBranch — есть ли локальная ветка refs/heads/<branch>.
// Код 0 — ветка есть; код 1 — нет, это отсев; иной код/сбой — ошибка.
func keepHasBranch(ctx context.Context, git GitExecutor, log func(string), dir, branch string) (bool, error) {
	leave := logging.Enter("branch", branch)
	ok, res, err := git.VerifyQuiet(ctx, dir, "refs/heads/"+branch)
	leave("ok", ok, "exit", res.Exit, "err", err)
	logResult(log, res)
	if err != nil {
		if res.Exit == 1 {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}

// keepOnBranch — стоит ли копия на ветке <branch>.
// Сравнение имени после refs/heads/ точное.
// Код 1 — отсев. Иной ненулевой код и сбой запуска — ошибка критерия.
func keepOnBranch(ctx context.Context, git GitExecutor, log func(string), dir, branch string) (bool, error) {
	leave := logging.Enter("branch", branch)
	name, res, err := git.SymbolicHead(ctx, dir)
	leave("name", name, "exit", res.Exit, "err", err)
	logResult(log, res)
	if err != nil && res.Exit == 0 {
		return false, err
	}
	if res.Exit == 1 {
		return false, nil
	}
	if res.Exit != 0 {
		return false, criterionExit(res)
	}
	return name == branch, nil
}

// keepNoRemoteOnHost — указывает ли хоть один URL ремота на host.
// Ремотов нет или ни один URL не на host — копия остаётся.
func keepNoRemoteOnHost(ctx context.Context, git GitExecutor, log func(string), dir string, host Host) (bool, error) {
	leave := logging.Enter("host", host.Name, "port", host.Port)
	names, res, err := git.RemoteNames(ctx, dir)
	leave("names", len(names), "exit", res.Exit, "err", err)
	logResult(log, res)
	if err != nil {
		return false, err
	}
	for _, name := range names {
		keep, err := remoteMissesHost(ctx, git, log, dir, name, host)
		if err != nil || !keep {
			return keep, err
		}
	}
	return true, nil
}

func remoteMissesHost(ctx context.Context, git GitExecutor, log func(string), dir, name string, host Host) (bool, error) {
	seen := map[string]struct{}{}
	for _, read := range []func() (string, bool, string, error){
		func() (string, bool, string, error) { return git.RemoteURL(ctx, dir, name) },
		func() (string, bool, string, error) { return git.RemotePushURL(ctx, dir, name) },
	} {
		raw, missing, output, err := read()
		logText(log, output)
		if err != nil {
			return false, err
		}
		if missing {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		got, ok := hostOfRemote(raw)
		if ok && SameHost(got, host) {
			return false, nil
		}
	}
	return true, nil
}

func criterionExit(res gitexec.Result) error {
	text := strings.TrimSpace(res.Stderr)
	if text == "" {
		return fmt.Errorf("git exited %d", res.Exit)
	}
	return fmt.Errorf("git exited %d: %s", res.Exit, text)
}

func logResult(log func(string), res gitexec.Result) {
	logText(log, res.Stderr)
	logText(log, res.Stdout)
}

func logText(log func(string), text string) {
	if log == nil || text == "" {
		return
	}
	log(text)
}
