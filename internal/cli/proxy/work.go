// Одна рабочая копия: чтение ремотов при токене и один пользовательский git.
package proxy

import (
	"context"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/gitexec"
)

type copyJob struct {
	dir string
	rel string
}

func runCopies(ctx context.Context, jobs []copyJob, verb string, tail []string, auth gitexec.HTTPSAuth, git GitRunner, logf func(string), steps *console.GitStep, print func(string), abort bool) bool {
	bad := false
	for _, job := range jobs {
		ok, note := runCopy(ctx, job, verb, tail, auth, git, logf, steps)
		line := formatResult(ok, job.rel, note)
		if steps != nil {
			steps.Result(print, line)
		} else if print != nil {
			print(line)
		}
		if ok {
			continue
		}
		bad = true
		if abort {
			return true
		}
	}
	return bad
}

func runCopy(ctx context.Context, job copyJob, verb string, tail []string, auth gitexec.HTTPSAuth, git GitRunner, logf func(string), steps *console.GitStep) (bool, string) {
	cfg, note, failed := remoteConfig(ctx, job.dir, auth, git, logf)
	if failed {
		return false, note
	}
	args := make([]string, 0, len(cfg)+1+len(tail))
	args = append(args, cfg...)
	args = append(args, verb)
	args = append(args, tail...)
	if steps != nil {
		steps.Start(verb, job.rel, 1)
	}
	res, err := git.Run(ctx, job.dir, args)
	if steps != nil {
		steps.Stop()
	}
	logGit(logf, res)
	if failedRun(res, err) {
		return false, shortReason(res)
	}
	return true, ""
}

func remoteConfig(ctx context.Context, dir string, auth gitexec.HTTPSAuth, git GitRunner, logf func(string)) ([]string, string, bool) {
	if strings.TrimSpace(auth.Token) == "" {
		return nil, "", false
	}
	listed, err := git.Run(ctx, dir, []string{"remote"})
	logGit(logf, listed)
	if failedRun(listed, err) {
		return nil, shortReason(listed), true
	}
	var cfg []string
	seen := map[string]struct{}{}
	for _, name := range nonEmptyLines(listed.Stdout) {
		for _, extra := range [][]string{{name}, {"--push", name}} {
			args := append([]string{"remote", "get-url"}, extra...)
			got, gerr := git.Run(ctx, dir, args)
			logGit(logf, got)
			if failedRun(got, gerr) {
				return nil, shortReason(got), true
			}
			raw := strings.TrimSpace(storedURL(got.Stdout))
			if raw == "" {
				continue
			}
			if _, ok := seen[raw]; ok {
				continue
			}
			pair, perr := auth.InsteadOf(raw)
			if perr != nil {
				if logf != nil {
					logf(perr.Error())
				}
				return nil, "git failed", true
			}
			if len(pair) == 0 {
				continue
			}
			seen[raw] = struct{}{}
			cfg = append(cfg, pair...)
		}
	}
	return cfg, "", false
}

func storedURL(out string) string {
	out = strings.TrimRight(out, "\r\n")
	if i := strings.IndexAny(out, "\r\n"); i >= 0 {
		out = out[:i]
	}
	return out
}

func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func logGit(logf func(string), res gitexec.Result) {
	if logf == nil {
		return
	}
	logf(res.Stdout)
	logf(res.Stderr)
}
