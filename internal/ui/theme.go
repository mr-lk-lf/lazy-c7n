// Package ui holds the visual theme shared by every screen.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Styles is the whole palette, derived once per terminal background.
type Styles struct {
	Dark bool

	Brand       lipgloss.Style
	Tab         lipgloss.Style
	ActiveTab   lipgloss.Style
	Pane        lipgloss.Style // unfocused pane
	FocusedPane lipgloss.Style
	PaneTitle   lipgloss.Style
	Item        lipgloss.Style
	Bold        lipgloss.Style
	Muted       lipgloss.Style
	Cursor      lipgloss.Style // row under the cursor, focused pane
	CursorDim   lipgloss.Style // row under the cursor, other pane
	Marked      lipgloss.Style
	DryBadge    lipgloss.Style
	LiveBadge   lipgloss.Style
	Status      lipgloss.Style
	StatusError lipgloss.Style
	OK          lipgloss.Style

	// Syntax highlighting (YAML, JSON).
	Key     lipgloss.Style
	String  lipgloss.Style
	Number  lipgloss.Style
	Comment lipgloss.Style

	// Live-run gate.
	GatePane lipgloss.Style
	Danger   lipgloss.Style // destructive actions, deploy warnings
	Warn     lipgloss.Style
	Input    lipgloss.Style
}

// Palette colors. Live/destructive is always red; dry-run always green.
var (
	accentLight = lipgloss.Color("#5A56E0")
	accentDark  = lipgloss.Color("#7D79F6")
	mutedLight  = lipgloss.Color("#8A8A8A")
	mutedDark   = lipgloss.Color("#6C6C6C")
	borderLight = lipgloss.Color("#C8C8D0")
	borderDark  = lipgloss.Color("#3E3E50")
	textLight   = lipgloss.Color("#1C1C24")
	textDark    = lipgloss.Color("#E4E4EC")
	cursorLight = lipgloss.Color("#E2E1FB")
	cursorDark  = lipgloss.Color("#34325E")
	dimLight    = lipgloss.Color("#ECECF0")
	dimDark     = lipgloss.Color("#26263A")
	greenLight  = lipgloss.Color("#1F8A55")
	greenDark   = lipgloss.Color("#4CC38A")
	cyanLight   = lipgloss.Color("#0B7A8C")
	cyanDark    = lipgloss.Color("#5FC9DB")
	green       = lipgloss.Color("#2EB872")
	red         = lipgloss.Color("#E5484D")
	amberLight  = lipgloss.Color("#B26A00")
	amberDark   = lipgloss.Color("#F5A524")
	white       = lipgloss.Color("#FFFFFF")
	black       = lipgloss.Color("#101014")
)

func NewStyles(dark bool) Styles {
	ld := lipgloss.LightDark(dark)
	accent := ld(accentLight, accentDark)
	muted := ld(mutedLight, mutedDark)
	text := ld(textLight, textDark)
	border := ld(borderLight, borderDark)
	amber := ld(amberLight, amberDark)
	okGreen := ld(greenLight, greenDark)

	pill := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(bg).Foreground(fg)
	}
	pane := func(c color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 1)
	}

	return Styles{
		Dark:        dark,
		Brand:       pill(accent, white),
		Tab:         lipgloss.NewStyle().Padding(0, 1).Foreground(muted),
		ActiveTab:   lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(accent).Underline(true),
		Pane:        pane(border),
		FocusedPane: pane(accent),
		PaneTitle:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		Item:        lipgloss.NewStyle().Foreground(text),
		Bold:        lipgloss.NewStyle().Bold(true).Foreground(text),
		Muted:       lipgloss.NewStyle().Foreground(muted),
		Cursor:      lipgloss.NewStyle().Background(ld(cursorLight, cursorDark)).Bold(true),
		CursorDim:   lipgloss.NewStyle().Background(ld(dimLight, dimDark)),
		Marked:      lipgloss.NewStyle().Bold(true).Foreground(accent),
		DryBadge:    pill(green, black),
		LiveBadge:   pill(red, white),
		Status:      lipgloss.NewStyle().Foreground(muted),
		StatusError: lipgloss.NewStyle().Bold(true).Foreground(red),
		OK:          lipgloss.NewStyle().Foreground(okGreen),

		Key:     lipgloss.NewStyle().Foreground(accent),
		String:  lipgloss.NewStyle().Foreground(okGreen),
		Number:  lipgloss.NewStyle().Foreground(ld(cyanLight, cyanDark)),
		Comment: lipgloss.NewStyle().Foreground(muted).Italic(true),

		GatePane: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(red).
			Padding(0, 1),
		Danger: lipgloss.NewStyle().Bold(true).Foreground(red),
		Warn:   lipgloss.NewStyle().Foreground(amber),
		Input:  lipgloss.NewStyle().Bold(true).Foreground(text),
	}
}
