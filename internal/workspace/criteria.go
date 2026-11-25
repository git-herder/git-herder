// Реализация критериев отбора: проверка одной копии через git.
package workspace

import (
	"context"

	"github.com/git-herder/git-herder/internal/diag/logging"
)

// keepHasBranch — есть ли локальная ветка refs/heads/<branch>.
// Код 0 — ветка есть; код 1 — нет, это отсев; иной код/сбой — ошибка.
func keepHasBranch(ctx context.Context, dir, branch string) (bool, error) {
	leave := logging.Enter("branch", branch)
	ok, res, err := git.VerifyQuiet(ctx, dir, "refs/heads/"+branch)
	leave("ok", ok, "exit", res.Exit, "err", err)
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
// Код 0 с другим именем, код 1 или отсутствие символической ссылки — отсев.
func keepOnBranch(ctx context.Context, dir, branch string) (bool, error) {
	leave := logging.Enter("branch", branch)
	name, res, err := git.SymbolicHead(ctx, dir)
	leave("name", name, "exit", res.Exit, "err", err)
	if err != nil && res.Exit == 0 {
		return false, err
	}
	if res.Exit != 0 {
		return false, nil
	}
	return name == branch, nil
}

// keepNoRemoteOnHost — указывает ли хоть один URL ремота на host.
// Ремотов нет или ни один URL не на host — копия остаётся.
func keepNoRemoteOnHost(ctx context.Context, dir string, host Host) (bool, error) {
	leave := logging.Enter("host", host.Name, "port", host.Port)
	urls, output, err := git.Remotes(ctx, dir)
	leave("urls", len(urls), "err", err)
	logging.Debug("git remote", "reason", "git", "output", output)
	if err != nil {
		return false, err
	}
	for _, raw := range urls {
		got, ok := hostOfRemote(raw)
		if ok && SameHost(got, host) {
			return false, nil
		}
	}
	return true, nil
}
