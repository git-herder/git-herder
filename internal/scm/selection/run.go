// Пайплайн CommonScmSelection: листинг, обогащение, клиентский фильтр.
package selection

import (
	"context"
	"errors"
	"fmt"

	"github.com/git-herder/git-herder/internal/diag/logging"
	"github.com/git-herder/git-herder/internal/resolved"
	"github.com/git-herder/git-herder/internal/retrying"
	"github.com/git-herder/git-herder/internal/scm"
	"github.com/git-herder/git-herder/internal/scm/card"
	"github.com/git-herder/git-herder/internal/scm/conn"
	"github.com/git-herder/git-herder/internal/scm/factory"
)

// Options — повторы, политика и прогресс одного отбора.
type Options struct {
	Retries int
	OnError resolved.OnError
	Meter   Meter
}

// ErrObjectFailed — continue исчерпал повторы задания branches хотя бы у одной карточки.
var ErrObjectFailed = errors.New("one or more repositories failed during selection")

// Run выполняет отбор и возвращает карточки после клиентской фильтрации.
// objectFailed — при continue хотя бы одно задание branches исчерпало повторы:
// такие карточки в набор не входят. При --on-error=continue и ошибке страницы
// листинга результат пустой, objectFailed остаётся false.
func Run(ctx context.Context, f *factory.ClientFactory, cfg conn.Config, crit scm.Criteria, opt Options) (cards []card.Card, objectFailed bool, err error) {
	leave := logging.Enter("ctx", ctx, "host", cfg.Host, "upstream", cfg.Upstream, "search", crit.Search, "retries", opt.Retries, "onError", opt.OnError)
	defer func() { leave("cards", cards, "objectFailed", objectFailed, "err", err) }()
	if opt.Meter == nil {
		opt.Meter = NopMeter{}
	}
	listing, jobs, err := f.Open(cfg, crit)
	if err != nil {
		return nil, false, err
	}
	policy := retrying.PolicyContinue
	if opt.OnError == resolved.OnErrorAbort {
		policy = retrying.PolicyAbort
	}
	fatalList := fatal403
	if crit.Search {
		fatalList = nil
	}
	cards, err = listAll(ctx, listing, opt.Retries, policy, fatalList, opt.Meter, cfg.Upstream, crit)
	if err != nil {
		return nil, false, err
	}
	if crit.Search || len(cards) == 0 || len(jobs) == 0 {
		if crit.Search {
			return cards, false, nil
		}
		return Filter(cfg.Upstream, cards, crit), false, nil
	}
	cards = filterEarly(cfg.Upstream, cards, crit, jobs)
	if len(cards) == 0 {
		return cards, false, nil
	}
	cards, objectFailed, err = enrichAll(ctx, cards, jobs, opt.Retries, policy, opt.Meter)
	if err != nil {
		return nil, false, err
	}
	return filterLate(cfg.Upstream, cards, crit, jobs), objectFailed, nil
}

func listAll(ctx context.Context, listing scm.Listing, retries int, policy retrying.Policy, fatal func(error) bool, meter Meter, up resolved.Upstream, crit scm.Criteria) ([]card.Card, error) {
	var pages int
	_, err := retrying.Execute([]int{0}, retries, policy, fatal, func(int) error {
		n, err := listing.PagesCount(ctx)
		if err != nil {
			return wrapList403(up, crit, err)
		}
		pages = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	if pages < 1 {
		return []card.Card{}, nil
	}
	ids := make([]int, pages)
	for i := range ids {
		ids[i] = i + 1
	}
	slots := make([][]card.Card, pages+1)
	meter.Start("Listing repositories", pages)
	defer meter.Stop()
	failed, err := retrying.Execute(ids, retries, policy, fatal, func(page int) error {
		cards, err := listing.Repositories(ctx, page)
		if err != nil {
			err = wrapList403(up, crit, err)
			logging.Debug("listing page", "page", page, "err", err)
			return err
		}
		slots[page] = cards
		meter.Advance()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if failed > 0 {
		return []card.Card{}, nil
	}
	var out []card.Card
	for i := 1; i <= pages; i++ {
		out = append(out, slots[i]...)
	}
	if out == nil {
		out = []card.Card{}
	}
	return out, nil
}

func enrichAll(ctx context.Context, cards []card.Card, jobs []scm.Enricher, retries int, policy retrying.Policy, meter Meter) ([]card.Card, bool, error) {
	type pair struct {
		idx int
		job scm.Enricher
	}
	work := make([]pair, 0, len(cards)*len(jobs))
	for i := range cards {
		for _, job := range jobs {
			work = append(work, pair{idx: i, job: job})
		}
	}
	meter.Start("Enriching repositories", len(work))
	defer meter.Stop()
	branchFailed := map[int]bool{}
	_, err := retrying.Execute(work, retries, policy, nil, func(p pair) error {
		err := p.job.Apply(ctx, &cards[p.idx])
		if err != nil {
			if p.job.ID() == "branches" {
				branchFailed[p.idx] = true
			}
			logging.Debug("enrich", "id", p.job.ID(), "fullName", cards[p.idx].FullName(), "err", err)
			return err
		}
		if p.job.ID() == "branches" {
			delete(branchFailed, p.idx)
		}
		meter.Advance()
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if len(branchFailed) == 0 {
		return cards, false, nil
	}
	out := make([]card.Card, 0, len(cards)-len(branchFailed))
	for i, c := range cards {
		if branchFailed[i] {
			continue
		}
		out = append(out, c)
	}
	return out, true, nil
}

func fatal403(err error) bool {
	var he *scm.HTTPError
	return errors.As(err, &he) && he.Status == 403
}

func wrapList403(up resolved.Upstream, crit scm.Criteria, err error) error {
	if !fatal403(err) || crit.Search {
		return err
	}
	switch up {
	case resolved.UpstreamGitHub:
		return fmt.Errorf("GitHub list returned 403; check token rights for repository search: %w", err)
	case resolved.UpstreamGitLab:
		if len(crit.EqualAttrs()) > 0 {
			return fmt.Errorf("GitLab list returned 403; custom attributes filter may require sufficient permissions: %w", err)
		}
		return fmt.Errorf("GitLab list returned 403: %w", err)
	default:
		return err
	}
}
