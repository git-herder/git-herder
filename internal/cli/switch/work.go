package switchcmd

import (
	"context"
	"strings"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/gitexec"
)

type memory struct {
	hadBranch bool
	branch    string
	hadOID    bool
	oid       string
	dirty     bool
}

func switchCopies(ctx context.Context, jobs []copyJob, s cli.SwitchSettings, git gitOps, logf func(string), steps *console.GitStep, print func(string), abort bool) bool {
	bad := false
	for _, j := range jobs {
		sw, up, notes := handleSwitch(ctx, j.dir, s, git, logf, steps, j.rel)
		if steps != nil {
			steps.Result(print, formatSwitch(sw, up, j.rel, notes))
		} else if print != nil {
			print(formatSwitch(sw, up, j.rel, notes))
		}
		if badSign(sw) || badSign(up) {
			bad = true
			if abort {
				break
			}
		}
	}
	return bad
}

type copyJob struct {
	dir string
	rel string
}

func handleSwitch(ctx context.Context, dir string, s cli.SwitchSettings, git gitOps, logf func(string), steps *console.GitStep, label string) (string, string, []string) {
	mem, ok := rememberSwitch(ctx, dir, s, git, logf)
	if !ok {
		return signFail, signSkip, []string{"проверка копии не удалась", "не начинался"}
	}
	if steps != nil {
		steps.Start("switch", label, 1)
	}
	req := gitexec.SwitchRequest{
		NoGuess:     true,
		Discard:     s.DiscardChanges,
		Merge:       s.Merge,
		Create:      s.Create,
		ForceCreate: s.ForceCreate,
		Opts:        append([]string{}, s.GitOpts...),
		Target:      s.Branch,
	}
	if s.StartPointSet {
		req.StartPoint = s.StartPoint
	}
	out, err := git.Switch(ctx, dir, req)
	if steps != nil {
		steps.Stop()
	}
	logOut(logf, out)

	if s.Merge {
		names, uout, uerr := git.Unmerged(ctx, dir)
		logOut(logf, uout)
		if uerr != nil {
			return signFail, signSkip, []string{"проверка копии не удалась", "не начинался"}
		}
		if note := conflictNote(names); note != "" {
			return signConflict, signSkip, []string{note}
		}
	}
	if err != nil {
		return signFail, signSkip, []string{shortReason(out, "переключение не удалось"), "не начинался"}
	}

	name, nout, nerr := git.SymbolicHead(ctx, dir)
	logOut(logf, nout)
	if nerr != nil {
		return signFail, signSkip, []string{"проверка копии не удалась", "не начинался"}
	}
	oid, missing, hout, herr := git.HeadCommit(ctx, dir)
	logOut(logf, hout)
	if herr != nil {
		return signFail, signSkip, []string{"проверка копии не удалась", "не начинался"}
	}
	if name != s.Branch {
		return signFail, signSkip, []string{"переключение не удалось", "не начинался"}
	}
	changed := !mem.hadBranch || mem.branch != name
	if mem.hadOID != !missing || (mem.hadOID && oid != mem.oid) {
		changed = true
	}
	if s.DiscardChanges && mem.dirty {
		changed = true
	}
	sw := signSame
	if changed {
		sw = signOK
	}
	up, note := settleUpstream(ctx, dir, s, git, logf)
	if note == "" {
		return sw, up, nil
	}
	return sw, up, []string{note}
}

func rememberSwitch(ctx context.Context, dir string, s cli.SwitchSettings, git gitOps, logf func(string)) (memory, bool) {
	name, out, err := git.SymbolicHead(ctx, dir)
	logOut(logf, out)
	if err != nil {
		return memory{}, false
	}
	oid, missing, out, err := git.HeadCommit(ctx, dir)
	logOut(logf, out)
	if err != nil {
		return memory{}, false
	}
	mem := memory{hadBranch: name != "", branch: name, hadOID: !missing, oid: oid}
	if !s.DiscardChanges {
		return mem, true
	}
	dirty, out, err := git.Porcelain(ctx, dir)
	logOut(logf, out)
	if err != nil {
		return memory{}, false
	}
	mem.dirty = dirty
	return mem, true
}

func settleUpstream(ctx context.Context, dir string, s cli.SwitchSettings, git gitOps, logf func(string)) (string, string) {
	if !s.TrackSet {
		return signSame, ""
	}
	out, err := git.BranchUpstream(ctx, dir, s.Branch)
	logOut(logf, out)
	if launchFailed(out, err) {
		return signFail, "проверка копии не удалась"
	}
	want := s.Track + "/" + s.Branch
	if line, one := oneLine(out.Stdout); err == nil && one && line == want {
		return signSame, ""
	}
	out, err = git.SetUpstream(ctx, dir, s.Track, s.Branch)
	logOut(logf, out)
	if err != nil {
		return signFail, shortReason(out, "вышестоящая ветка не записана")
	}
	return signOK, ""
}

func logOut(logf func(string), out gitOut) {
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
