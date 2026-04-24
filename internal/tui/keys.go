package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Start        key.Binding
	Stop         key.Binding
	Restart      key.Binding
	ToggleFollow key.Binding
	Quit         key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:           key.NewBinding(key.WithKeys("k", "up")),
	Down:         key.NewBinding(key.WithKeys("j", "down")),
	Start:        key.NewBinding(key.WithKeys("S")),
	Stop:         key.NewBinding(key.WithKeys("s")),
	Restart:      key.NewBinding(key.WithKeys("r")),
	ToggleFollow: key.NewBinding(key.WithKeys("f")),
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c")),
}
