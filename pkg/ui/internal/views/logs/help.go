package logsview

import "charm.land/bubbles/v2/key"

// ShortHelp returns the short help bindings for the logs view.
func (m *LogsView) ShortHelp() []key.Binding {
	return m.KeyMap.ShortHelp()
}

// FullHelp returns the full help bindings for the logs view.
func (m *LogsView) FullHelp() [][]key.Binding {
	return m.KeyMap.FullHelp()
}
