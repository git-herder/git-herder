package merge

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/git-herder/git-herder/internal/cli"
	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/gitexec"
)

type copyResult struct {
	signs    [5]string
	notes    []string
	precheck bool
}

func (r copyResult) failed() bool {
	if r.precheck {
		return true
	}
	for _, sign := range r.signs {
		if badSign(sign) {
			return true
		}
	}
	return false
}

func mergeCopies(ctx context.Context, jobs []copyJob, s cli.MergeSettings, auth gitexec.HTTPSAuth, git gitOps, logf func(string), steps *console.GitStep, print func(string), abort bool) bool {
	bad := false
	for _, j := range jobs {
		res := handleMerge(ctx, j.dir, s, auth, git, logf, steps, j.rel)
		line := formatMerge(res.signs, j.rel, res.notes)
		if steps != nil {
			steps.Result(print, line)
		} else if print != nil {
			print(line)
		}
		if res.failed() {
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

func handleMerge(ctx context.Context, dir string, s cli.MergeSettings, auth gitexec.HTTPSAuth, git gitOps, logf func(string), steps *console.GitStep, label string) copyResult {
	entryBranch, detached, entryOID, stop := rememberEntry(ctx, dir, git, logf)
	if stop != "" {
		return prechecked(stop)
	}
	if note, ok := guardCopy(ctx, dir, git, logf); !ok {
		return prechecked(note)
	}

	cfg, err := fetchConfig(ctx, dir, s.Remote, auth, git, logf)
	if err != nil {
		return at(0, "fetch failed")
	}
	if steps != nil {
		steps.Start("fetch", label, 1)
	}
	fout, ferr := git.Fetch(ctx, dir, cfg, joinOpts(s.GitOpts, s.FetchOpts), s.Remote)
	if steps != nil {
		steps.Stop()
	}
	logOut(logf, fout)
	if ferr != nil {
		return at(0, shortReason(fout, "fetch failed"))
	}

	swReq := gitexec.SwitchRequest{Opts: joinOpts(s.GitOpts, s.SwitchOpts), Target: s.To}
	sout, serr := git.Switch(ctx, dir, swReq)
	logOut(logf, sout)
	if serr != nil {
		return placed(signOK, signFail, signSkip, signSkip, signSkip, shortReason(sout, "switch failed"))
	}
	name, nout, nerr := git.SymbolicHead(ctx, dir)
	logOut(logf, nout)
	changed := detached || name != entryBranch
	if nerr != nil {
		res := placed(signOK, signFail, signSkip, signSkip, signSkip, "working copy check failed")
		ret, note := restore(ctx, dir, s, detached, entryBranch, entryOID, git, logf)
		res.signs[4] = ret
		if note != "" {
			res.notes = append(res.notes, note)
		}
		return res
	}
	swSign := signOK
	if !changed {
		swSign = signSame
	}
	headAfter, missing, hout, herr := git.HeadCommit(ctx, dir)
	logOut(logf, hout)
	if herr != nil || missing {
		res := placed(signOK, signFail, signSkip, signSkip, signSkip, "working copy check failed")
		if changed {
			ret, note := restore(ctx, dir, s, detached, entryBranch, entryOID, git, logf)
			res.signs[4] = ret
			if note != "" {
				res.notes = append(res.notes, note)
			}
		}
		return res
	}

	if steps != nil {
		steps.Start("merge", label, 1)
	}
	mout, merr := git.MergeBranch(ctx, dir, joinOpts(s.GitOpts, s.MergeOpts), s.From)
	if steps != nil {
		steps.Stop()
	}
	logOut(logf, mout)
	pending, files, cerr := mergePending(ctx, git, dir, logf)
	if cerr != nil {
		return placed(signOK, swSign, signFail, signSkip, signSkip, "working copy check failed")
	}
	if pending {
		return placed(signOK, swSign, signConflict, signSkip, signSkip, conflictNote(files))
	}
	if merr != nil {
		res := placed(signOK, swSign, signFail, signSkip, signSkip, shortReason(mout, "merge failed"))
		if changed {
			ret, note := restore(ctx, dir, s, detached, entryBranch, entryOID, git, logf)
			res.signs[4] = ret
			if note != "" {
				res.notes = append(res.notes, note)
			}
		} else {
			res.signs[4] = signSame
		}
		return res
	}
	now, missing, aout, aerr := git.HeadCommit(ctx, dir)
	logOut(logf, aout)
	if aerr != nil || missing {
		return placed(signOK, swSign, signFail, signSkip, signSkip, "working copy check failed")
	}
	mergeSign := signSame
	if now != headAfter {
		mergeSign = signOK
	}
	return postMerge(ctx, dir, s, detached, entryBranch, entryOID, swSign, mergeSign, git, logf)
}

func rememberEntry(ctx context.Context, dir string, git gitOps, logf func(string)) (branch string, detached bool, oid, stop string) {
	name, out, err := git.SymbolicHead(ctx, dir)
	logOut(logf, out)
	if launchFailed(out, err) {
		return "", false, "", "working copy check failed"
	}
	if name != "" {
		return name, false, "", ""
	}
	oid, out, err = git.HeadVerify(ctx, dir)
	logOut(logf, out)
	if launchFailed(out, err) {
		return "", false, "", "working copy check failed"
	}
	oid = strings.TrimSpace(oid)
	if err != nil || oid == "" {
		return "", false, "", "no commit at HEAD"
	}
	return "", true, oid, ""
}

func guardCopy(ctx context.Context, dir string, git gitOps, logf func(string)) (string, bool) {
	ok, out, err := git.VerifyQuiet(ctx, dir, "MERGE_HEAD")
	logOut(logf, out)
	if err != nil {
		return "working copy check failed", false
	}
	if ok {
		return "merge already in progress", false
	}
	ok, out, err = git.VerifyQuiet(ctx, dir, "CHERRY_PICK_HEAD")
	logOut(logf, out)
	if err != nil {
		return "working copy check failed", false
	}
	if ok {
		return "cherry-pick already in progress", false
	}
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		path, gout, gerr := git.GitPath(ctx, dir, name)
		logOut(logf, gout)
		if gerr != nil {
			return "working copy check failed", false
		}
		if dirExists(dir, path) {
			return "rebase already in progress", false
		}
	}
	dirty, out, err := git.Porcelain(ctx, dir)
	logOut(logf, out)
	if err != nil {
		return "working copy check failed", false
	}
	if dirty {
		return "uncommitted changes", false
	}
	return "", true
}

func postMerge(ctx context.Context, dir string, s cli.MergeSettings, detached bool, entryBranch, entryOID, swSign, mergeSign string, git gitOps, logf func(string)) copyResult {
	remain := s.To
	if s.Stay {
		if detached {
			remain = ""
		} else {
			remain = entryBranch
		}
	}
	del := signSame
	var notes []string
	if !s.KeepSource && remain != s.From {
		out, err := git.DeleteBranch(ctx, dir, joinOpts(s.GitOpts, s.BranchOpts), s.From)
		logOut(logf, out)
		if err != nil {
			del = signFail
			notes = append(notes, shortReason(out, "branch delete failed"))
		} else {
			del = signOK
		}
	}
	ret := signSame
	if s.Stay && (detached || entryBranch != s.To) {
		sign, note := restore(ctx, dir, s, detached, entryBranch, entryOID, git, logf)
		ret = sign
		if note != "" {
			notes = append(notes, note)
		}
	}
	res := placed(signOK, swSign, mergeSign, del, ret)
	res.notes = notes
	return res
}

func restore(ctx context.Context, dir string, s cli.MergeSettings, detached bool, entryBranch, entryOID string, git gitOps, logf func(string)) (string, string) {
	req := gitexec.SwitchRequest{Opts: joinOpts(s.GitOpts, s.SwitchOpts)}
	if detached {
		req.Detach = true
		req.Target = entryOID
	} else {
		req.Target = entryBranch
	}
	out, err := git.Switch(ctx, dir, req)
	logOut(logf, out)
	if err != nil {
		return signFail, shortReason(out, "return failed")
	}
	return signOK, ""
}

func fetchConfig(ctx context.Context, dir, remote string, auth gitexec.HTTPSAuth, git gitOps, logf func(string)) ([]string, error) {
	if strings.TrimSpace(auth.Token) == "" {
		return nil, nil
	}
	raw, missing, output, err := git.RemoteURL(ctx, dir, remote)
	if logf != nil {
		logf(output)
	}
	if err != nil || missing || strings.TrimSpace(raw) == "" {
		return nil, errString("fetch failed")
	}
	return auth.InsteadOf(raw)
}

func mergePending(ctx context.Context, git gitOps, dir string, logf func(string)) (bool, []string, error) {
	ok, out, err := git.MergeHead(ctx, dir)
	logOut(logf, out)
	names, uout, uerr := git.Unmerged(ctx, dir)
	logOut(logf, uout)
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

func prechecked(note string) copyResult {
	return copyResult{
		signs:    [5]string{signSkip, signSkip, signSkip, signSkip, signSkip},
		notes:    []string{note},
		precheck: true,
	}
}

func at(i int, note string) copyResult {
	signs := [5]string{signSkip, signSkip, signSkip, signSkip, signSkip}
	signs[i] = signFail
	return copyResult{signs: signs, notes: []string{note}}
}

func placed(fetch, sw, mergeSign, del, ret string, notes ...string) copyResult {
	res := copyResult{signs: [5]string{fetch, sw, mergeSign, del, ret}}
	for _, note := range notes {
		if note != "" {
			res.notes = append(res.notes, note)
		}
	}
	return res
}

func dirExists(root, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
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

type errString string

func (e errString) Error() string { return string(e) }
