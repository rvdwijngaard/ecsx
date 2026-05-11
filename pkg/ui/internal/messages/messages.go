package messages

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
	ClusterName string
}

type ToggleHelp struct{}

type ToggleNotificationDialog struct {
	Msg   string
	Error error
}
