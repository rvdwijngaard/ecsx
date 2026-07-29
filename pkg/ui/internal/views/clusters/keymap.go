package clusterselection

import "charm.land/bubbles/v2/key"

// DetailsPaneKeyMap defines keybindings for the details pane.
type DetailsPaneKeyMap struct {
	Zoom key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *DetailsPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Zoom}
}

// FullHelp implements the KeyMap interface.
func (km *DetailsPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Zoom},
	}
}

// DefaultDetailsKeyMap returns a default set of keybindings.
func DefaultDetailsKeyMap() *DetailsPaneKeyMap {
	return &DetailsPaneKeyMap{
		Zoom: key.NewBinding(
			key.WithKeys("Z"),
			key.WithHelp("shift+z", "zoom"),
		),
	}
}

// ------------------------------------------ //

// ClusterPaneKeyMap defines keybindings for the cluster selection pane.
type ClusterPaneKeyMap struct {
	Select      key.Binding
	Search      key.Binding
	Zoom        key.Binding
	Copy        key.Binding
	OpenConsole key.Binding
	HostShell   key.Binding
	Reload      key.Binding
	Esc         key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *ClusterPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Select, km.Search, km.Zoom, km.OpenConsole, km.HostShell, km.Reload, km.Esc}
}

// FullHelp implements the KeyMap interface.
func (km *ClusterPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Select, km.Search, km.Zoom, km.Esc, km.Reload, km.Copy, km.OpenConsole, km.HostShell},
	}
}

// DefaultClusterPaneKeyMap returns a default set of keybindings.
func DefaultClusterPaneKeyMap() *ClusterPaneKeyMap {
	return &ClusterPaneKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Zoom: key.NewBinding(
			key.WithKeys("Z"),
			key.WithHelp("shift+z", "zoom"),
		),
		Copy: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("shift+y", "copy"),
		),
		OpenConsole: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "console"),
		),
		HostShell: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "host shell"),
		),
		Reload: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reload"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel search"),
		),
	}
}

// ------------------------------------------ //

// ClusterViewKeyMap defines keybindings for the cluster selection view.
type ClusterViewKeyMap struct {
	MoveFocus key.Binding
	Regions   key.Binding
}

// DialogKeyMaps collects keys that toggle view-specific dialogs.
type DialogKeyMaps struct {
	RegionDialog key.Binding
}

func (m *ClusterSelection) DialogKeyMaps() DialogKeyMaps {
	return DialogKeyMaps{
		RegionDialog: m.KeyMap.Regions,
	}
}

// ShortHelp implements the KeyMap interface.
func (km *ClusterViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.MoveFocus, km.Regions}
}

// FullHelp implements the KeyMap interface.
func (km *ClusterViewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.MoveFocus, km.Regions},
	}
}

// DefaultClusterViewKeyMap returns a default set of keybindings.
func DefaultClusterViewKeyMap() *ClusterViewKeyMap {
	return &ClusterViewKeyMap{
		MoveFocus: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab/shift+tab", "switch panes"),
		),
		Regions: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("shift+r", "region select"),
		),
	}
}
