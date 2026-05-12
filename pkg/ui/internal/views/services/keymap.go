package serviceselection

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

// ServicePaneKeyMap defines keybindings for the service selection pane.
type ServicePaneKeyMap struct {
	Select key.Binding
	Search key.Binding
	Zoom   key.Binding
	Copy   key.Binding
	Reload key.Binding
	Esc    key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *ServicePaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Select, km.Search, km.Zoom, km.Reload, km.Esc}
}

// FullHelp implements the KeyMap interface.
func (km *ServicePaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Select, km.Search, km.Zoom, km.Esc, km.Reload, km.Copy},
	}
}

// DefaultServicePaneKeyMap returns a default set of keybindings.
func DefaultServicePaneKeyMap() *ServicePaneKeyMap {
	return &ServicePaneKeyMap{
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
		Reload: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reload"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
	}
}

// ------------------------------------------ //

// ServiceViewKeyMap defines keybindings for the service selection view.
type ServiceViewKeyMap struct {
	MoveFocus key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *ServiceViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.MoveFocus}
}

// FullHelp implements the KeyMap interface.
func (km *ServiceViewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.MoveFocus},
	}
}

// DefaultServiceViewKeyMap returns a default set of keybindings.
func DefaultServiceViewKeyMap() *ServiceViewKeyMap {
	return &ServiceViewKeyMap{
		MoveFocus: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab/shift+tab", "switch panes"),
		),
	}
}
