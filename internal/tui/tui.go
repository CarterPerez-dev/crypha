/*
©AngelaMos | 2026
tui.go

Program entry that constructs and runs the interactive bubbletea wizard
*/

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func Run() error {
	_, err := tea.NewProgram(New(), tea.WithAltScreen()).Run()
	return err
}
