package logsview

import "charm.land/bubbles/v2/key"

// LogsKeyMap defines keybindings for the logs view.
type LogsKeyMap struct {
	Esc    key.Binding
	Top    key.Binding
	Bottom key.Binding
	Up     key.Binding
	Down   key.Binding
	PageUp key.Binding
	PageDn key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *LogsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Esc, km.Up, km.Down, km.Bottom}
}

// FullHelp implements the KeyMap interface.
func (km *LogsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Esc, km.Up, km.Down, km.PageUp, km.PageDn, km.Top, km.Bottom},
	}
}

// DefaultLogsKeyMap returns the default keybindings for the logs view.
func DefaultLogsKeyMap() *LogsKeyMap {
	return &LogsKeyMap{
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close logs"),
		),
		Top: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom (resume auto-scroll)"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDn: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
	}
}
