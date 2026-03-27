package tui

import "github.com/charmbracelet/bubbletea"

type KeyMap struct{}

var DefaultKeyMap = KeyMap{}

func (k KeyMap) ShortHelp() []tea.Key {
	return nil
}

func (k KeyMap) FullHelp() [][]tea.Key {
	return nil
}
