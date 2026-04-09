package ui

import "charm.land/lipgloss/v2"

var (
	paneBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69"))
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69")).
			PaddingLeft(1)
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			PaddingLeft(2)
)
