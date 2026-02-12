package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all the keybindings for the application
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Enter         key.Binding
	Tab           key.Binding
	ShiftTab      key.Binding
	Pane1         key.Binding
	Pane2         key.Binding
	Search        key.Binding
	SearchContent key.Binding
	Escape        key.Binding
	Quit          key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	Home          key.Binding
	End           key.Binding
	BracketLeft   key.Binding
	BracketRight  key.Binding
	PaneLeft      key.Binding
	PaneRight     key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "collapse folder"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "expand folder"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "switch pane back"),
		),
		Pane1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "files pane"),
		),
		Pane2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "diff pane"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search files"),
		),
		SearchContent: key.NewBinding(
			key.WithKeys("\\"),
			key.WithHelp("\\", "search content"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close/back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		BracketLeft: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev tab"),
		),
		BracketRight: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next tab"),
		),
		PaneLeft: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "left pane"),
		),
		PaneRight: key.NewBinding(
			key.WithKeys("ctrl+h"),
			key.WithHelp("ctrl+h", "right pane"),
		),
	}
}

// HelpKeys returns the keys to show in help
func (k KeyMap) HelpKeys() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Tab, k.Search, k.Enter, k.Quit,
	}
}
