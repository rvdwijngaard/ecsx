package taskselection

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

// TaskPaneKeyMap defines keybindings for the task selection pane.
type TaskPaneKeyMap struct {
	Search key.Binding
	Zoom   key.Binding
	Copy   key.Binding
	Reload key.Binding
	Esc    key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *TaskPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Search, km.Zoom, km.Reload, km.Esc}
}

// FullHelp implements the KeyMap interface.
func (km *TaskPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Search, km.Zoom, km.Esc, km.Reload, km.Copy},
	}
}

// DefaultTaskPaneKeyMap returns a default set of keybindings.
func DefaultTaskPaneKeyMap() *TaskPaneKeyMap {
	return &TaskPaneKeyMap{
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

// TaskViewKeyMap defines keybindings for the task selection view.
type TaskViewKeyMap struct {
	MoveFocus key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km *TaskViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.MoveFocus}
}

// FullHelp implements the KeyMap interface.
func (km *TaskViewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.MoveFocus},
	}
}

// DefaultTaskViewKeyMap returns a default set of keybindings.
func DefaultTaskViewKeyMap() *TaskViewKeyMap {
	return &TaskViewKeyMap{
		MoveFocus: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab/shift+tab", "switch panes"),
		),
	}
}
