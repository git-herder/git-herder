// Пакет workspace: GitWorkingCopiesResolver ищет рабочие копии в корне
// --workspace и возвращает те, что прошли отбор.
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Snowbridge/git-herder/internal/diag/console"
	"github.com/Snowbridge/git-herder/internal/diag/logging"
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

// Keep решает, остаётся ли копия в наборе.
// false исключает копию. Ненулевая ошибка останавливает отбор.
type Keep func(Copy) (bool, error)

// Picker рисует интерактивный список. Метод совпадает с console.UI.Select.
type Picker interface {
	Select(items []console.SelectItem) ([]int, error)
}

// Request — уже собранные параметры одного вызова.
type Request struct {
	Root        string
	QP          []string
	QN          []string
	Keep        Keep
	Interactive bool
	Picker      Picker
}

// ErrCancelled — пользователь нажал ESC.
// Команда завершается с кодом 0 и не выполняет следующие шаги.
var ErrCancelled = fmt.Errorf("selection cancelled: %w", console.ErrSelectCancelled)

// Resolve обходит корень, применяет --qp и --qn, затем колбэк и интерактив.
// Ошибка чтения, колбэка или недоступный терминал не сопровождается списком.
// Пустой набор при включённом интерактиве не открывает экран.
func Resolve(req Request) (out []Copy, err error) {
	leave := logging.Enter(
		"root", req.Root,
		"qp", len(req.QP),
		"qn", len(req.QN),
		"interactive", req.Interactive,
	)
	defer func() { leave("out", len(out), "err", err) }()

	found, err := walk(req.Root)
	if err != nil {
		return nil, err
	}
	matched := make([]Copy, 0, len(found))
	for _, c := range found {
		if passFilters(c, req.QP, req.QN) {
			matched = append(matched, c)
		}
	}
	kept, err := applyKeep(matched, req.Keep)
	if err != nil {
		return nil, err
	}
	if !req.Interactive || len(kept) == 0 {
		return kept, nil
	}
	return pick(kept, req.Picker)
}

func applyKeep(in []Copy, keep Keep) ([]Copy, error) {
	if keep == nil {
		return in, nil
	}
	out := make([]Copy, 0, len(in))
	for _, c := range in {
		ok, err := keep(c)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func pick(kept []Copy, picker Picker) ([]Copy, error) {
	if picker == nil {
		return nil, fmt.Errorf("interactive picker is not set")
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
