package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/git-herder/git-herder/internal/gitexec"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/retrying"
)

type syncJob struct {
	dir      string
	rel      string
	attempts int
	bad      bool
}

func syncCopies(ctx context.Context, jobs []*syncJob, branch, remote string, pullOpts, pushOpts []string, auth gitexec.HTTPSAuth, retries int, onError resolved.OnError, git gitOps, logf func(string), print func(string)) (bad bool, err error) {
	if len(jobs) == 0 {
		return false, nil
	}
	policy := retrying.PolicyContinue
	if onError == resolved.OnErrorAbort {
		policy = retrying.PolicyAbort
	}
	_, runErr := retrying.Execute(jobs, retries, policy, nil, func(j *syncJob) error {
		return j.step(ctx, branch, remote, pullOpts, pushOpts, auth, retries, git, logf, print)
	})
	for _, j := range jobs {
		if j.bad {
			bad = true
		}
	}
	return bad, runErr
}

func (j *syncJob) step(ctx context.Context, branch, remote string, pullOpts, pushOpts []string, auth gitexec.HTTPSAuth, retries int, git gitOps, logf func(string), print func(string)) error {
	j.attempts++
	names, out, err := git.RemoteNames(ctx, j.dir)
	logGit(logf, out)
	if err != nil {
		return j.done(retries, nil, signFail, signFail, []string{shortReason(out, "получение не удалось"), "отправка не начиналась"}, print)
	}
	if !hasName(names, remote) {
		return j.done(retries, nil, signFail, signFail, []string{"ремота нет", "отправка не начиналась"}, print)
	}
	cfg, err := j.configArgs(ctx, remote, auth, git, logf)
	if err != nil {
		return j.done(retries, nil, signFail, signFail, []string{err.Error(), "отправка не начиналась"}, print)
	}
	remoteOID, found, out, err := git.LsRemoteHead(ctx, j.dir, remote, branch, cfg)
	logGit(logf, out)
	if out.Exit == 128 {
		return j.done(retries, queueErr(err, out, "получение не удалось"), signFail, signFail, []string{shortReason(out, "получение не удалось"), "отправка не начиналась"}, print)
	}
	if err != nil {
		return j.done(retries, nil, signFail, signFail, []string{shortReason(out, "получение не удалось"), "отправка не начиналась"}, print)
	}

	pullSign := signSame
	var pullNotes []string
	merged := false
	if found {
		before, bout, berr := git.HeadOID(ctx, j.dir)
		logGit(logf, bout)
		if berr != nil {
			return j.done(retries, nil, signFail, signFail, []string{shortReason(bout, "получение не удалось"), "отправка не начиналась"}, print)
		}
		pout, perr := git.Pull(ctx, j.dir, pullOpts, remote, branch, cfg)
		logGit(logf, pout)
		pending, files, merr := mergeState(ctx, git, j.dir, logf)
		if pending {
			merged = true
			pullSign = signConflict
			if note := conflictNote(files); note != "" {
				pullNotes = append(pullNotes, note)
			}
		} else if merr != nil {
			return j.done(retries, nil, signFail, signFail, []string{merr.Error(), "отправка не начиналась"}, print)
		} else if pout.Exit == 128 {
			return j.done(retries, queueErr(perr, pout, "получение не удалось"), signFail, signFail, []string{shortReason(pout, "получение не удалось"), "отправка не начиналась"}, print)
		} else if perr != nil {
			return j.done(retries, nil, signFail, signFail, []string{shortReason(pout, "получение не удалось"), "отправка не начиналась"}, print)
		} else {
			after, aout, aerr := git.HeadOID(ctx, j.dir)
			logGit(logf, aout)
			if aerr != nil {
				return j.done(retries, nil, signFail, signFail, []string{shortReason(aout, "получение не удалось"), "отправка не начиналась"}, print)
			}
			if after != before {
				pullSign = signIn
			}
		}
	}

	pushOut, pushErr := git.Push(ctx, j.dir, pushOpts, remote, branch, cfg)
	logGit(logf, pushOut)
	if pushOut.Exit == 128 && merged {
		notes := append(append([]string{}, pullNotes...), shortReason(pushOut, "отправка не удалась"))
		return j.done(retries, nil, pullSign, signFail, notes, print)
	}
	if pushOut.Exit == 128 {
		notes := append(append([]string{}, pullNotes...), shortReason(pushOut, "отправка не удалась"))
		return j.done(retries, queueErr(pushErr, pushOut, "отправка не удалась"), pullSign, signFail, notes, print)
	}
	if pushErr != nil {
		notes := append(append([]string{}, pullNotes...), shortReason(pushOut, "отправка не удалась"))
		return j.done(retries, nil, pullSign, signFail, notes, print)
	}
	head, hout, herr := git.HeadOID(ctx, j.dir)
	logGit(logf, hout)
	if herr != nil {
		notes := append(append([]string{}, pullNotes...), shortReason(hout, "отправка не удалась"))
		return j.done(retries, nil, pullSign, signFail, notes, print)
	}
	pushSign := signSame
	if remoteOID == "" || head != remoteOID {
		pushSign = signIn
	}
	return j.done(retries, nil, pullSign, pushSign, pullNotes, print)
}

func (j *syncJob) configArgs(ctx context.Context, remote string, auth gitexec.HTTPSAuth, git gitOps, logf func(string)) ([]string, error) {
	if strings.TrimSpace(auth.Token) == "" {
		return nil, nil
	}
	raw, missing, output, err := git.RemoteURL(ctx, j.dir, remote)
	if logf != nil {
		logf(output)
	}
	if err != nil {
		return nil, err
	}
	if missing || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("ремота нет")
	}
	return auth.InsteadOf(raw)
}

func (j *syncJob) done(retries int, qerr error, pull, push string, notes []string, print func(string)) error {
	if qerr == nil || j.attempts > retries {
		j.bad = badSign(pull) || badSign(push)
		print(formatResult(pull, push, j.rel, notes))
	}
	return qerr
}

func mergeState(ctx context.Context, git gitOps, dir string, logf func(string)) (bool, []string, error) {
	ok, out, err := git.MergeHead(ctx, dir)
	logGit(logf, out)
	names, uout, uerr := git.Unmerged(ctx, dir)
	logGit(logf, uout)
	if ok || (uerr == nil && len(names) > 0) {
		if uerr != nil {
			names = nil
		}
		return true, names, nil
	}
	if err != nil {
		return false, nil, err
	}
	if uerr != nil {
		return false, nil, uerr
	}
	return false, nil, nil
}

func queueErr(err error, out gitOut, fallback string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", shortReason(out, fallback))
}

func hasName(names []string, remote string) bool {
	for _, name := range names {
		if name == remote {
			return true
		}
	}
	return false
}

func logGit(logf func(string), out gitOut) {
	if logf == nil {
		return
	}
	text := out.Stderr
	if out.Stdout != "" {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += out.Stdout
	}
	logf(text)
}
