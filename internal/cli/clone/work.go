package clone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Snowbridge/git-herder/internal/resolved"
	"github.com/Snowbridge/git-herder/internal/retrying"
	"github.com/Snowbridge/git-herder/internal/scm/card"
)

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
}

func cloneCards(ctx context.Context, workspace string, cards []card.Card, preserve bool, protocol string, pol resolvedExisting, opts []string, retries int, onError resolved.OnError, git gitFunc, logf func(string), print func(string)) (failed bool, err error) {
	policy := retrying.PolicyContinue
	if onError == resolved.OnErrorAbort {
		policy = retrying.PolicyAbort
	}
	var jobs []*cloneJob
	for _, c := range cards {
		dest, destErr := destination(workspace, c.FullName(), preserve)
		if destErr != nil {
			print(failLine(c.FullName(), destErr.Error()))
			failed = true
			if policy == retrying.PolicyAbort {
				return true, destErr
			}
			continue
		}
		kind, statErr := inspect(dest)
		if statErr != nil {
			return failed, statErr
		}
		if kind == placeFile {
			msg := "destination path is a file"
			print(failLine(c.FullName(), msg))
			failed = true
			if policy == retrying.PolicyAbort {
				return true, fmt.Errorf("%s", msg)
			}
			continue
		}
		addr, addrErr := repoURL(c.URL, protocol)
		if addrErr != nil {
			print(failLine(c.FullName(), addrErr.Error()))
			failed = true
			if policy == retrying.PolicyAbort {
				return true, addrErr
			}
			continue
		}
		jobs = append(jobs, &cloneJob{card: c, dest: dest, repoURL: addr, place: kind})
	}
	if len(jobs) == 0 {
		return failed, nil
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
			print(skipLine(j.card.FullName()))
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
	out, err := git(ctx, opts, j.repoURL, j.dest)
	if logf != nil && out != "" {
		logf(out)
	}
	if err != nil {
		if st, statErr := os.Stat(j.dest); statErr == nil && st.IsDir() {
			j.cleanup = true
		}
		return j.finish(retries, err, shortReason(out), print)
	}
	print(okLine(j.card.FullName()))
	return nil
}

func (j *cloneJob) finish(retries int, err error, reason string, print func(string)) error {
	if j.attempts > retries {
		if strings.TrimSpace(reason) == "" {
			reason = failFallback
		}
		print(failLine(j.card.FullName(), reason))
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
