// Package ui holds the visual theme shared by every screen.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Styles is the whole palette, derived once per terminal background.
type Styles struct {
	Dark bool

	Brand     lipgloss.Style
	Tab       lipgloss.Style
	ActiveTab lipgloss.Style
	Pane      lipgloss.Style
	PaneTitle lipgloss.Style
	Item      lipgloss.Style
	Muted     lipgloss.Style
	DryBadge  lipgloss.Style
	LiveBadge lipgloss.Style
	Status    lipgloss.Style

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

	pill := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(bg).Foreground(fg)
	}

	return Styles{
		Dark:      dark,
		Brand:     pill(accent, white),
		Tab:       lipgloss.NewStyle().Padding(0, 1).Foreground(muted),
		ActiveTab: lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(accent).Underline(true),
		Pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1),
		PaneTitle: lipgloss.NewStyle().Bold(true).Foreground(accent),
		Item:      lipgloss.NewStyle().Foreground(text),
		Muted:     lipgloss.NewStyle().Foreground(muted),
		DryBadge:  pill(green, black),
		LiveBadge: pill(red, white),
		Status:    lipgloss.NewStyle().Foreground(muted),
		GatePane: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(red).
			Padding(0, 1),
		Danger: lipgloss.NewStyle().Bold(true).Foreground(red),
		Warn:   lipgloss.NewStyle().Foreground(ld(amberLight, amberDark)),
		Input:  lipgloss.NewStyle().Bold(true).Foreground(text),
	}
}
