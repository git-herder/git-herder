// Пакет workspace: GitWorkingCopiesResolver ищет рабочие копии в корне
// --workspace, применяет --qp, --qn, критерии и интерактивный отбор.
// Критерии накапливаются текучим API и исполняются через GitCliRunner.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/git-herder/git-herder/internal/diag/console"
	"github.com/git-herder/git-herder/internal/diag/logging"
)

// Copy — рабочая копия: каталог, в котором лежит папка .git.
type Copy struct {
	// Dir — абсолютный путь каталога без завершающего разделителя.
	Dir string
}

// Name — последний компонент Dir.
func (c Copy) Name() string {
	return filepath.Base(c.Dir)
}

// Path — каталог, в котором лежит копия.
func (c Copy) Path() string {
	return filepath.Dir(c.Dir)
}

// Picker рисует интерактивный список. Метод совпадает с console.UI.Select.
type Picker interface {
	Select(items []console.SelectItem) ([]int, error)
}

// criterion — накопленный критерий: проверка одной копии в её каталоге.
// false отбрасывает копию; ошибка останавливает отбор.
type criterion func(ctx context.Context, c Copy) (bool, error)

// Resolver накапливает условия отбора и исполняет их в Find.
type Resolver struct {
	root        string
	paths       []string
	names       []string
	criteria    []criterion
	git         GitExecutor
	log         func(string)
	interactive bool
	picker      Picker
}

// ErrCancelled — пользователь нажал ESC.
// Команда завершается с кодом 0 и не выполняет следующие шаги.
var ErrCancelled = fmt.Errorf("selection cancelled: %w", console.ErrSelectCancelled)

// Resolve — точка входа текучего API: резолвер без аргументов.
func Resolve() *Resolver {
	return &Resolver{}
}

// Workspace задаёт абсолютный путь корня --workspace.
func (r *Resolver) Workspace(absPath string) *Resolver {
	r.root = absPath
	return r
}

// Path добавляет повторяемые условия --qp на путь копии.
func (r *Resolver) Path(qp ...string) *Resolver {
	r.paths = append(r.paths, qp...)
	return r
}

// Name добавляет повторяемые условия --qn на имя копии.
func (r *Resolver) Name(qn ...string) *Resolver {
	r.names = append(r.names, qn...)
	return r
}

// Git задаёт исполнителя критериев. Пустой аргумент уже заданного не затирает.
// Без вызова критерии идут в GitCliRunner.
func (r *Resolver) Git(g GitExecutor) *Resolver {
	if g != nil {
		r.git = g
	}
	return r
}

// Log принимает stderr и stdout каждого вызова git критерия.
// Пустую строку резолвер не передаёт. Без вызова вывод в журнал не пишется.
func (r *Resolver) Log(fn func(string)) *Resolver {
	r.log = fn
	return r
}

// Interactive включает интерактивный отбор после критериев.
func (r *Resolver) Interactive(on bool) *Resolver {
	r.interactive = on
	return r
}

// Picker задаёт источник интерактивного списка. Вызывается только при Interactive.
func (r *Resolver) Picker(p Picker) *Resolver {
	r.picker = p
	return r
}

// HasBranch оставляет копии с локальной веткой <branch>.
// Отсутствие ветки — отсев, не ошибка.
func (r *Resolver) HasBranch(branch string) *Resolver {
	r.criteria = append(r.criteria, func(ctx context.Context, c Copy) (bool, error) {
		return keepHasBranch(ctx, r.executor(), r.log, c.Dir, branch)
	})
	return r
}

// OnBranch оставляет копии, стоящие на ветке <branch> сейчас.
// Отделённый HEAD и другая ветка — отсев, не ошибка.
func (r *Resolver) OnBranch(branch string) *Resolver {
	r.criteria = append(r.criteria, func(ctx context.Context, c Copy) (bool, error) {
		return keepOnBranch(ctx, r.executor(), r.log, c.Dir, branch)
	})
	return r
}

// NoRemoteOnHost отбрасывает копии, у которых хоть один URL ремота
// указывает на host. Совпадение hostname и порта — точное.
func (r *Resolver) NoRemoteOnHost(host Host) *Resolver {
	r.criteria = append(r.criteria, func(ctx context.Context, c Copy) (bool, error) {
		return keepNoRemoteOnHost(ctx, r.executor(), r.log, c.Dir, host)
	})
	return r
}

func (r *Resolver) executor() GitExecutor {
	if r.git != nil {
		return r.git
	}
	return execGit{}
}

// Find запускает обход, фильтры, критерии и интерактив и возвращает список.
// Повторный вызов на том же резолвере контрактом не описан.
func (r *Resolver) Find(ctx context.Context) (out []Copy, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	leave := logging.Enter(
		"root", r.root,
		"qp", len(r.paths),
		"qn", len(r.names),
		"criteria", len(r.criteria),
		"interactive", r.interactive,
	)
	defer func() { leave("out", len(out), "err", err) }()

	found, err := walk(r.root)
	if err != nil {
		return nil, err
	}
	matched := make([]Copy, 0, len(found))
	for _, c := range found {
		if passFilters(c, r.paths, r.names) {
			matched = append(matched, c)
		}
	}
	kept, err := applyCriteria(ctx, matched, r.criteria)
	if err != nil {
		return nil, err
	}
	if r.interactive && len(kept) > 0 {
		kept, err = pick(kept, r.picker)
		if err != nil {
			return nil, err
		}
	}
	if len(kept) == 0 {
		console.New().WarningStderr("no matching working copies")
	}
	return kept, nil
}

// applyCriteria прогоняет копии в порядке обнаружения.
// Первый отбрасывающий критерий исключает копию, ошибка останавливает отбор.
func applyCriteria(ctx context.Context, in []Copy, criteria []criterion) ([]Copy, error) {
	if len(criteria) == 0 {
		return in, nil
	}
	out := make([]Copy, 0, len(in))
	for _, c := range in {
		keep := true
		for _, check := range criteria {
			ok, err := check(ctx, c)
			if err != nil {
				return nil, err
			}
			if !ok {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, c)
		}
	}
	return out, nil
}

func pick(kept []Copy, picker Picker) ([]Copy, error) {
	if picker == nil {
		return nil, fmt.Errorf("interactive selection source is not set")
	}
	items := make([]console.SelectItem, len(kept))
	for i, c := range kept {
		items[i] = console.SelectItem{Text: c.Dir}
	}
	idx, err := picker.Select(items)
	if err != nil {
		if errors.Is(err, console.ErrSelectCancelled) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	selected := make([]Copy, 0, len(idx))
	for _, i := range idx {
		if i < 0 || i >= len(kept) {
			return nil, fmt.Errorf("interactive selection index %d is out of range", i)
		}
		selected = append(selected, kept[i])
	}
	return selected, nil
}
