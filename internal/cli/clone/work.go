package clone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/retrying"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/github"
)

// propertyReader — чтение одного атрибута. Реализует github.PropertyReader.
type propertyReader interface {
	ReadProperty(ctx context.Context, owner, repo, property string) (github.PropertyValue, error)
}

const failFallback = "клонирование не удалось"

type gitFunc func(ctx context.Context, opts []string, repoURL, dest string) (string, error)

type placeKind int

const (
	placeMissing placeKind = iota
	placeEmpty
	placeFull
	placeFile
)

type cloneJob struct {
	card     card.Card
	dest     string
	repoURL  string
	place    placeKind
	dropped  bool
	cleanup  bool
	attempts int
	steps    *console.GitStep
}

func cloneCards(ctx context.Context, workspace string, cards []card.Card, preserve bool, protocol string, pol resolvedExisting, opts []string, retries int, onError resolved.OnError, git gitFunc, logf func(string), steps *console.GitStep, print func(string), attrName string, props propertyReader) (failed bool, err error) {
	policy := retrying.PolicyContinue
	if onError == resolved.OnErrorAbort {
		policy = retrying.PolicyAbort
	}
	var jobs []*cloneJob
	consider := func(c card.Card, dest string, destErr error) error {
		if destErr != nil {
			print(failLine(c.FullName(), destErr.Error()))
			failed = true
			if policy == retrying.PolicyAbort {
				return destErr
			}
			return nil
		}
		kind, statErr := inspect(dest)
		if statErr != nil {
			return statErr
		}
		if kind == placeFile {
			msg := "destination path is a file"
			print(failLine(c.FullName(), msg))
			failed = true
			if policy == retrying.PolicyAbort {
				return fmt.Errorf("%s", msg)
			}
			return nil
		}
		addr, addrErr := repoURL(c.URL, protocol)
		if addrErr != nil {
			print(failLine(c.FullName(), addrErr.Error()))
			failed = true
			if policy == retrying.PolicyAbort {
				return addrErr
			}
			return nil
		}
		jobs = append(jobs, &cloneJob{card: c, dest: dest, repoURL: addr, place: kind})
		return nil
	}
	if attrName == "" {
		for _, c := range cards {
			dest, destErr := destination(workspace, c.FullName(), preserve)
			if err := consider(c, dest, destErr); err != nil {
				return true, err
			}
		}
	} else {
		ready, readFailed, err := readAttributes(ctx, cards, attrName, props, retries, policy, print)
		if readFailed {
			failed = true
		}
		if err != nil {
			return true, err
		}
		for _, item := range ready {
			var dest string
			var destErr error
			switch item.val.State {
			case github.PropertyString:
				dest, destErr = attributeDestination(workspace, item.val.Value)
			case github.PropertyAbsent:
				dest = workspace
			default:
				destErr = github.ErrPropertyNotString
			}
			if err := consider(item.card, dest, destErr); err != nil {
				return true, err
			}
		}
	}
	if len(jobs) == 0 {
		return failed, nil
	}
	steps.SetTotal(len(jobs))
	for _, j := range jobs {
		j.steps = steps
	}
	n, runErr := retrying.Execute(jobs, retries, policy, nil, func(j *cloneJob) error {
		return j.step(ctx, pol, opts, retries, git, logf, print)
	})
	if runErr != nil {
		return true, runErr
	}
	if n > 0 {
		failed = true
	}
	return failed, nil
}

type attrJob struct {
	card     card.Card
	val      github.PropertyValue
	attempts int
	ok       bool
}

func readAttributes(ctx context.Context, cards []card.Card, attrName string, props propertyReader, retries int, policy retrying.Policy, print func(string)) (ready []*attrJob, failed bool, err error) {
	var queued []*attrJob
	for _, c := range cards {
		if c.Name == "" || c.Path == "" || strings.Contains(c.Path, "/") {
			print(failLine(c.FullName(), scm.ErrContract.Error()))
			failed = true
			if policy == retrying.PolicyAbort {
				return nil, true, scm.ErrContract
			}
			continue
		}
		queued = append(queued, &attrJob{card: c})
	}
	if len(queued) == 0 {
		return nil, failed, nil
	}
	n, runErr := retrying.Execute(queued, retries, policy, nil, func(j *attrJob) error {
		return j.fetch(ctx, props, attrName, retries, print)
	})
	if runErr != nil {
		return nil, true, runErr
	}
	if n > 0 {
		failed = true
	}
	for _, j := range queued {
		if j.ok {
			ready = append(ready, j)
		}
	}
	return ready, failed, nil
}

func (j *attrJob) fetch(ctx context.Context, props propertyReader, name string, retries int, print func(string)) error {
	j.attempts++
	v, err := props.ReadProperty(ctx, j.card.Path, j.card.Name, name)
	if err != nil {
		if j.attempts > retries {
			print(failLine(j.card.FullName(), err.Error()))
		}
		return err
	}
	j.val = v
	j.ok = true
	return nil
}

type resolvedExisting int

const (
	existSkip resolvedExisting = iota
	existDrop
)

func (j *cloneJob) step(ctx context.Context, pol resolvedExisting, opts []string, retries int, git gitFunc, logf func(string), print func(string)) error {
	j.attempts++
	if j.cleanup {
		if err := os.RemoveAll(j.dest); err != nil {
			return j.finish(retries, err, err.Error(), print)
		}
		j.cleanup = false
		j.place = placeMissing
	}
	if j.place == placeFull && !j.dropped {
		if pol == existSkip {
			j.steps.Result(print, skipLine(j.card.FullName()))
			return nil
		}
		if err := os.RemoveAll(j.dest); err != nil {
			return j.finish(retries, err, err.Error(), print)
		}
		j.dropped = true
		j.place = placeMissing
	}
	if err := os.MkdirAll(filepath.Dir(j.dest), 0o755); err != nil {
		return j.finish(retries, err, err.Error(), print)
	}
	j.steps.Start("clone", j.card.FullName(), j.attempts)
	out, err := git(ctx, opts, j.repoURL, j.dest)
	j.steps.Stop()
	if logf != nil && out != "" {
		logf(out)
	}
	if err != nil {
		if st, statErr := os.Stat(j.dest); statErr == nil && st.IsDir() {
			j.cleanup = true
		}
		return j.finish(retries, err, shortReason(out), print)
	}
	j.steps.Result(print, okLine(j.card.FullName()))
	return nil
}

func (j *cloneJob) finish(retries int, err error, reason string, print func(string)) error {
	if j.attempts > retries {
		if strings.TrimSpace(reason) == "" {
			reason = failFallback
		}
		j.steps.Result(print, failLine(j.card.FullName(), reason))
	}
	return err
}

func inspect(path string) (placeKind, error) {
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return placeMissing, nil
	}
	if err != nil {
		return 0, err
	}
	if !st.IsDir() {
		return placeFile, nil
	}
	empty, err := dirEmpty(path)
	if err != nil {
		return 0, err
	}
	if empty {
		return placeEmpty, nil
	}
	return placeFull, nil
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

func okLine(name string) string   { return "✔ " + name }
func skipLine(name string) string { return "➡️ " + name }
func failLine(name, reason string) string {
	return "✖ " + name + ": " + reason
}

func shortReason(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return failFallback
}
