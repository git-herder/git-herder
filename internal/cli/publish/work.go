package publish

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/retrying"
	"github.com/git-herder/git-herder/internal/scm"
)

const pushFallback = "отправка не удалась"

type resultKind int

const (
	resultNone resultKind = iota
	resultOK
	resultSkip
	resultFail
	resultWarn
)

type propAPI interface {
	WriteProperty(ctx context.Context, owner, repo, property, value string) error
}

type publishJob struct {
	dir      string
	rel      string
	path     string
	name     string
	attr     string
	addr     string
	attempts int
	result   resultKind
	steps    *console.GitStep
}

var unixNow = func() int64 { return time.Now().Unix() }

func publishCopies(ctx context.Context, jobs []*publishJob, remote string, existing cli.PublishExisting, attrName string, retries int, onError resolved.OnError, git gitOps, creator scm.RepositoryCreator, writer propAPI, logf func(string), steps *console.GitStep, print func(string)) (failed bool, err error) {
	steps.SetTotal(len(jobs))
	if len(jobs) == 0 {
		return false, nil
	}
	for _, j := range jobs {
		j.steps = steps
	}
	policy := retrying.PolicyContinue
	if onError == resolved.OnErrorAbort {
		policy = retrying.PolicyAbort
	}
	n, runErr := retrying.Execute(jobs, retries, policy, nil, func(j *publishJob) error {
		return j.step(ctx, remote, existing, attrName, retries, git, creator, writer, logf, print)
	})
	if runErr != nil {
		return true, runErr
	}
	if n > 0 {
		failed = true
	}
	for _, j := range jobs {
		if j.result == resultFail || j.result == resultWarn {
			failed = true
		}
	}
	return failed, nil
}

func (j *publishJob) step(ctx context.Context, remote string, existing cli.PublishExisting, attrName string, retries int, git gitOps, creator scm.RepositoryCreator, writer propAPI, logf func(string), print func(string)) error {
	j.attempts++
	exists, err := creator.Exists(ctx, j.path, j.name)
	if err != nil {
		return j.finish(retries, err, resultFail, failLine(j.rel, reasonOf("", err)), print)
	}
	if !exists {
		if err = creator.Create(ctx, j.path, j.name); err != nil {
			return j.finish(retries, err, resultFail, failLine(j.rel, reasonOf("", err)), print)
		}
	}
	if attrName != "" && writer != nil {
		if err = writer.WriteProperty(ctx, j.path, j.name, attrName, j.attr); err != nil {
			return j.finish(retries, err, resultFail, failLine(j.rel, reasonOf("", err)), print)
		}
	}
	skip, err := bindRemote(ctx, git, j.dir, remote, j.addr, existing, logf, j.steps, j.rel, j.attempts)
	if err != nil {
		return j.finish(retries, err, resultFail, failLine(j.rel, reasonOf("", err)), print)
	}
	if skip {
		j.result = resultSkip
		j.steps.Result(print, skipLine(j.rel))
		return nil
	}
	def, cur, err := chooseBranches(ctx, git, j.dir, logf)
	if err != nil {
		return j.finish(retries, err, resultFail, failLine(j.rel, reasonOf("", err)), print)
	}
	pushed := 0
	want := 1
	var failOut string
	var failErr error
	j.steps.Start("push "+def, j.rel, j.attempts)
	out, err := git.Push(ctx, j.dir, remote, def)
	j.steps.Stop()
	logText(logf, out)
	if err == nil {
		pushed++
	} else {
		failOut, failErr = out, err
	}
	if cur != def {
		want = 2
		j.steps.Start("push "+cur, j.rel, j.attempts)
		out, err = git.Push(ctx, j.dir, remote, cur)
		j.steps.Stop()
		logText(logf, out)
		if err == nil {
			pushed++
		} else if failErr == nil {
			failOut, failErr = out, err
		}
	}
	if pushed == want {
		j.result = resultOK
		j.steps.Result(print, okLine(j.rel))
		return nil
	}
	kind := resultFail
	line := failLine(j.rel, reasonOf(failOut, failErr))
	if want == 2 && pushed == 1 {
		kind = resultWarn
		line = warnLine(j.rel, reasonOf(failOut, failErr))
	}
	return j.finish(retries, fmt.Errorf("%s", reasonOf(failOut, failErr)), kind, line, print)
}

func (j *publishJob) finish(retries int, err error, kind resultKind, line string, print func(string)) error {
	if j.attempts > retries {
		j.result = kind
		j.steps.Result(print, line)
	}
	return err
}

func bindRemote(ctx context.Context, git gitOps, dir, remote, want string, policy cli.PublishExisting, logf func(string), steps *console.GitStep, label string, attempt int) (bool, error) {
	got, missing, out, err := git.RemoteURL(ctx, dir, remote)
	logText(logf, out)
	if missing {
		steps.Start("remote", label, attempt)
		out, err = git.RemoteAdd(ctx, dir, remote, want)
		steps.Stop()
		logText(logf, out)
		return false, err
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(got) == strings.TrimSpace(want) {
		return false, nil
	}
	if policy == cli.PublishSkip {
		return true, nil
	}
	renamed := fmt.Sprintf("%s_%d", remote, unixNow())
	steps.Start("remote", label, attempt)
	out, err = git.RemoteRename(ctx, dir, remote, renamed)
	steps.Stop()
	logText(logf, out)
	if err != nil {
		return false, err
	}
	steps.Start("remote", label, attempt)
	out, err = git.RemoteAdd(ctx, dir, remote, want)
	steps.Stop()
	logText(logf, out)
	return false, err
}

func chooseBranches(ctx context.Context, git gitOps, dir string, logf func(string)) (string, string, error) {
	cur, detached, out, err := git.Head(ctx, dir)
	logText(logf, out)
	if err != nil {
		return "", "", err
	}
	if detached {
		return "", "", fmt.Errorf("detached HEAD")
	}
	for _, name := range []string{"main", "master"} {
		ok, bout, err := git.HasBranch(ctx, dir, name)
		logText(logf, bout)
		if err != nil {
			return "", "", err
		}
		if ok {
			return name, cur, nil
		}
	}
	return cur, cur, nil
}

func logText(logf func(string), text string) {
	if logf != nil && strings.TrimSpace(text) != "" {
		logf(text)
	}
}

func reasonOf(output string, err error) string {
	if line := firstLine(output); line != "" {
		return line
	}
	if err != nil {
		if line := firstLine(err.Error()); line != "" {
			return line
		}
	}
	return pushFallback
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func okLine(rel string) string   { return "✔ " + rel }
func skipLine(rel string) string { return "➡️ " + rel }
func failLine(rel, reason string) string {
	return "✖ " + rel + ": " + reason
}
func warnLine(rel, reason string) string {
	return "⚠ " + rel + ": " + reason
}
