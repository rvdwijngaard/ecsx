package tasks

import (
	"charm.land/bubbles/v2/key"
)

// ListPaneKeyMap defines keybindings for the tasks list pane.
type ListPaneKeyMap struct {
	Search key.Binding
	Zoom   key.Binding
	Reload key.Binding
	Back   key.Binding
	Quit   key.Binding
}

func (km *ListPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Search, km.Zoom, km.Reload, km.Back, km.Quit}
}

func (km *ListPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Search, km.Zoom, km.Reload, km.Back, km.Quit},
	}
}

func DefaultListPaneKeyMap() *ListPaneKeyMap {
	return &ListPaneKeyMap{
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Zoom: key.NewBinding(
			key.WithKeys("Z"),
			key.WithHelp("shift+z", "zoom"),
		),
		Reload: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reload"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// DetailsPaneKeyMap defines keybindings for the task details pane.
type DetailsPaneKeyMap struct {
	Zoom key.Binding
}

func (km *DetailsPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Zoom}
}

func (km *DetailsPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Zoom},
	}
}

func DefaultDetailsPaneKeyMap() *DetailsPaneKeyMap {
	return &DetailsPaneKeyMap{
		Zoom: key.NewBinding(
			key.WithKeys("Z"),
			key.WithHelp("shift+z", "zoom"),
		),
	}
}

// ViewKeyMap defines keybindings for the top-level tasks view.
type ViewKeyMap struct {
	MoveFocus key.Binding
}

func (km *ViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.MoveFocus}
}

func (km *ViewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.MoveFocus},
	}
}

func DefaultViewKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		MoveFocus: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab", "switch panes"),
		),
	}
}
