// Пакет console — обёртка pterm для пользовательского вывода на английском.
package console

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// UI выводит сообщения пользователю через pterm.
type UI struct{}

// New создаёт UI.
func New() *UI {
	return &UI{}
}

// PrintLine пишет строку в stdout.
func (u *UI) PrintLine(s string) {
	fmt.Println(s)
}

// PrintLines выводит несколько строк (список профилей).
func (u *UI) PrintLines(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

// Error выводит ошибку пользователю.
func (u *UI) Error(msg string) {
	pterm.Error.Println(msg)
}

// Warning выводит предупреждение.
func (u *UI) Warning(msg string) {
	pterm.Warning.Println(msg)
}

// Success выводит успех.
func (u *UI) Success(msg string) {
	pterm.Success.Println(msg)
}

// HighlightDefault помечает строку default в TTY или добавляет (default).
func (u *UI) HighlightDefault(name string, isDefault bool) string {
	if !isDefault {
		return name
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return pterm.FgGreen.Sprintf("%s (default)", name)
	}
	return name + " (default)"
}

// ConfirmDrop спрашивает подтверждение удаления профиля; false — отмена.
func (u *UI) ConfirmDrop(profileName string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true, nil
	}
	prompt := fmt.Sprintf("Delete profile %q?", profileName)
	return pterm.DefaultInteractiveConfirm.
		WithDefaultText(prompt).
		WithDefaultValue(false).
		Show()
}
