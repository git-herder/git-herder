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

// WarningStderr пишет предупреждение в stderr, не смешивая его со stdout отчёта.
func (u *UI) WarningStderr(msg string) {
	pterm.Warning.WithWriter(os.Stderr).Println(msg)
}

// ConfirmRiskyEnumeration спрашивает подтверждение отбора без критериев.
// Без TTY возвращает ошибку: продолжать нельзя.
func (u *UI) ConfirmRiskyEnumeration() (bool, error) {
	msg := "No selection criteria: listing every repository may take a long time and put a high load on the SCM API"
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("%s; set criteria or run in an interactive terminal", msg)
	}
	u.WarningStderr(msg)
	return pterm.DefaultInteractiveConfirm.
		WithDefaultText("Continue?").
		WithDefaultValue(false).
		Show()
}

// ConfirmMass спрашивает согласие на публикацию, когда объектов больше порога.
// Без терминала на stdin или stdout возвращает ошибку: продолжать нельзя.
// false — явный отказ.
func (u *UI) ConfirmMass(n int) (bool, error) {
	return u.ConfirmMassPrompt(fmt.Sprintf("Continue publishing %d working copies?", n))
}

// ConfirmMassPrompt спрашивает согласие с заданной формулировкой.
// Формулировка включает число объектов. false — явный отказ.
func (u *UI) ConfirmMassPrompt(prompt string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return false, fmt.Errorf("confirmation is not available without a terminal")
	}
	u.WarningStderr(prompt)
	return pterm.DefaultInteractiveConfirm.
		WithDefaultText("Continue?").
		WithDefaultValue(false).
		Show()
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
