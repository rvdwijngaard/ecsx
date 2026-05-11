package messages

import (
	apitypes "github.com/ron/ecsx/pkg/ui/internal/adapters/ecs/types"
)

// View identifies which view is active.
type View int

const (
	ClusterSelection View = iota
	ServiceSelection
	TaskSelection
)

type SwitchView struct {
	OldView View
	NewView View
}

type SelectCluster struct {
	ClusterName    string
	ClusterDetails apitypes.ClusterItem
}

type ZoomToggleServiceSelectionPane struct{}
type ZoomToggleServiceDetailsPane struct{}
type ZoomToggleClusterSelectionPane struct{}
type ZoomToggleClusterDetailsPane struct{}

type PreviewItem struct {
	StyledItem string
	RawItem    string
}

type ClusterDetails struct {
	Details *apitypes.ClusterItem
}

type ToggleHelp struct{}

type ToggleNotificationDialog struct {
	Msg   string
	Error error
}

type InitColumnVisibility struct {
	TableARN   string
	AllColumns []string // matching by index
	Visible    []bool   // matching by index
}
