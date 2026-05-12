package clusterselection

import "charm.land/bubbles/v2/key"

// VIEW
func (m *ClusterSelection) ShortHelp() []key.Binding {
	ah := appendShortHelp
	switch m.focused {
	case clusterPaneID:
		return ah(ah(m.clusterPane.ShortHelp(), m.KeyMap.ShortHelp()), m.clusterPane.AddKeyMap.Bindings())
	case detailsPaneID:
		return ah(ah(m.detailsPane.ShortHelp(), m.KeyMap.ShortHelp()), m.detailsPane.AddKeyMap.Bindings())
	}
	return nil
}

// CLUSTER PANE
func (m *clusterSelectionPane) ShortHelp() []key.Binding {
	return appendShortHelp(m.content.KeyMap.ShortHelp(), m.KeyMap.ShortHelp())
}

// DETAILS PANE
func (m *detailsPane) ShortHelp() []key.Binding {
	km := m.content.KeyMap
	viewportHelp := []key.Binding{km.Up, km.Down, km.Left, km.Right}
	return appendShortHelp(viewportHelp, m.KeyMap.ShortHelp())
}

// VIEW
func (m *ClusterSelection) FullHelp() [][]key.Binding {
	switch m.focused {
	case clusterPaneID:
		return appendFullHelp(m.clusterPane.FullHelp(), m.KeyMap.FullHelp())
	case detailsPaneID:
		return appendFullHelp(m.detailsPane.FullHelp(), m.KeyMap.FullHelp())
	}
	return nil
}

// CLUSTER PANE
func (m *clusterSelectionPane) FullHelp() [][]key.Binding {
	return appendFullHelp(m.content.KeyMap.FullHelp(), m.KeyMap.FullHelp())
}

// DETAILS PANE
func (m *detailsPane) FullHelp() [][]key.Binding {
	km := m.content.KeyMap
	viewportHelp := []key.Binding{km.Up, km.Down, km.Left, km.Right, km.HalfPageUp, km.HalfPageDown, km.PageUp, km.PageDown}
	return appendFullHelp([][]key.Binding{viewportHelp}, m.KeyMap.FullHelp())
}

func appendShortHelp(help []key.Binding, extra []key.Binding) []key.Binding {
	res := make([]key.Binding, len(help)+len(extra))
	copy(res[:len(help)], help)
	copy(res[len(help):], extra)
	return res
}

func appendFullHelp(help [][]key.Binding, extra [][]key.Binding) [][]key.Binding {
	res := make([][]key.Binding, len(help)+len(extra))
	copy(res[:len(help)], help)
	copy(res[len(help):], extra)
	return res
}
