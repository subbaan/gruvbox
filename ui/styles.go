package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/subbaan/gruvbox/theme"
)

var (
	ColorPrimary, ColorSecondary, ColorAccent, ColorWarning lipgloss.Color
	ColorSuccess, ColorBg, ColorFg, ColorDim                lipgloss.Color
	ColorPanel, ColorBorder, ColorPlayback                  lipgloss.Color
	BaseStyle, HeaderStyle, TitleStyle, StatusStyle         lipgloss.Style
	ActiveStepStyle, StepStyle, SelectedStepStyle           lipgloss.Style
	HelpStyle, KeyStyle, ValueStyle                         lipgloss.Style
	PanelStyle, FocusedPanelStyle                           lipgloss.Style
)

func init() {
	ApplyTheme(theme.Bundled()["warm-dark"])
}

// ApplyTheme runs on the UI/control thread and rebuilds styles from one palette.
func ApplyTheme(scheme theme.Scheme) {
	r := scheme.Roles()
	ColorPrimary, ColorSecondary = lipgloss.Color(r.Primary), lipgloss.Color(r.Success)
	ColorAccent, ColorWarning = lipgloss.Color(r.ActiveStep), lipgloss.Color(r.Warning)
	ColorSuccess, ColorBg = lipgloss.Color(r.Success), lipgloss.Color(r.Background)
	ColorFg, ColorDim = lipgloss.Color(r.Text), lipgloss.Color(r.Secondary)
	ColorPanel, ColorBorder = lipgloss.Color(r.Panel), lipgloss.Color(r.Border)
	ColorPlayback = lipgloss.Color(r.Playback)

	// Base styles
	BaseStyle = lipgloss.NewStyle().
		Foreground(ColorFg).
		Background(ColorBg)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 1)

	// Title style
	TitleStyle = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Padding(0, 1)

	// Status bar style
	StatusStyle = lipgloss.NewStyle().
		Foreground(ColorFg).
		Background(ColorPanel).
		Padding(0, 1)

	// Active step style
	ActiveStepStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(scheme.TextOn(r.ActiveStep))).
		Background(ColorAccent).
		Bold(true).
		Padding(0, 1)

	// Step style
	StepStyle = lipgloss.NewStyle().
		Foreground(ColorFg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDim).
		Padding(0, 1)

	// Selected step style
	SelectedStepStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.SelectedText)).
		Background(lipgloss.Color(r.Focus)).
		Bold(true).
		Padding(0, 1)

	// Help style
	HelpStyle = lipgloss.NewStyle().
		Foreground(ColorDim).
		Padding(0, 1)

	// Key binding style
	KeyStyle = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	// Value style
	ValueStyle = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	// Panel style
	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		MarginRight(1)

	// Focused panel style
	FocusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		MarginRight(1)
}
