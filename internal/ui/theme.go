// Package ui holds the visual theme shared by every screen.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Styles is the whole palette, derived once per theme and background.
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
	Chip        lipgloss.Style // small label, e.g. a tag

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

// Palette is the handful of colours a theme defines. Safety colours keep
// their meaning in every theme: Red = live/destructive, Green = dry-run/ok,
// Amber = mutating/deploys.
type Palette struct {
	Accent   color.Color
	Text     color.Color
	Muted    color.Color
	Border   color.Color
	CursorBg color.Color // nil: the cursor row is drawn in reverse video
	DimBg    color.Color // cursor row of the unfocused pane; nil: underline
	ChipBg   color.Color
	Green    color.Color
	Red      color.Color
	Amber    color.Color
	Cyan     color.Color
	OnColor  color.Color // text on a coloured badge
}

// Theme names, in the order T cycles through them.
var ThemeNames = []string{"lazyc7n", "terminal", "catppuccin", "gruvbox", "everforest", "tokyonight", "dracula"}

// HasTheme reports whether name is a known theme.
func HasTheme(name string) bool {
	for _, n := range ThemeNames {
		if n == name {
			return true
		}
	}
	return false
}

// NextTheme is the theme after name in ThemeNames.
func NextTheme(name string) string {
	for i, n := range ThemeNames {
		if n == name {
			return ThemeNames[(i+1)%len(ThemeNames)]
		}
	}
	return ThemeNames[0]
}

// Hex is shorthand for a colour literal.
func hex(s string) color.Color { return lipgloss.Color(s) }

// PaletteFor returns the palette of a theme in its dark or light variant.
// Unknown names get the default theme; dracula only has a dark variant.
func PaletteFor(name string, dark bool) Palette {
	switch name {
	case "terminal":
		// ANSI colours: whatever palette the terminal is configured with.
		return Palette{
			Accent: lipgloss.Color("4"), Text: lipgloss.NoColor{}, Muted: lipgloss.Color("8"),
			Border: lipgloss.Color("8"), ChipBg: lipgloss.Color("8"),
			Green: lipgloss.Color("2"), Red: lipgloss.Color("1"), Amber: lipgloss.Color("3"),
			Cyan: lipgloss.Color("6"), OnColor: lipgloss.Color("0"),
		}
	case "catppuccin":
		if dark { // Mocha
			return Palette{
				Accent: hex("#cba6f7"), Text: hex("#cdd6f4"), Muted: hex("#7f849c"), Border: hex("#45475a"),
				CursorBg: hex("#45475a"), DimBg: hex("#313244"), ChipBg: hex("#313244"),
				Green: hex("#a6e3a1"), Red: hex("#f38ba8"), Amber: hex("#fab387"), Cyan: hex("#89dceb"),
				OnColor: hex("#11111b"),
			}
		}
		return Palette{ // Latte
			Accent: hex("#8839ef"), Text: hex("#4c4f69"), Muted: hex("#8c8fa1"), Border: hex("#bcc0cc"),
			CursorBg: hex("#ccd0da"), DimBg: hex("#e6e9ef"), ChipBg: hex("#e6e9ef"),
			Green: hex("#40a02b"), Red: hex("#d20f39"), Amber: hex("#fe640b"), Cyan: hex("#179299"),
			OnColor: hex("#eff1f5"),
		}
	case "gruvbox":
		if dark {
			return Palette{
				Accent: hex("#fabd2f"), Text: hex("#ebdbb2"), Muted: hex("#928374"), Border: hex("#504945"),
				CursorBg: hex("#504945"), DimBg: hex("#3c3836"), ChipBg: hex("#3c3836"),
				Green: hex("#b8bb26"), Red: hex("#fb4934"), Amber: hex("#fe8019"), Cyan: hex("#8ec07c"),
				OnColor: hex("#282828"),
			}
		}
		return Palette{
			Accent: hex("#b57614"), Text: hex("#3c3836"), Muted: hex("#928374"), Border: hex("#d5c4a1"),
			CursorBg: hex("#d5c4a1"), DimBg: hex("#ebdbb2"), ChipBg: hex("#ebdbb2"),
			Green: hex("#79740e"), Red: hex("#9d0006"), Amber: hex("#af3a03"), Cyan: hex("#427b58"),
			OnColor: hex("#fbf1c7"),
		}
	case "everforest":
		if dark {
			return Palette{
				Accent: hex("#a7c080"), Text: hex("#d3c6aa"), Muted: hex("#859289"), Border: hex("#475258"),
				CursorBg: hex("#475258"), DimBg: hex("#343f44"), ChipBg: hex("#3d484d"),
				Green: hex("#a7c080"), Red: hex("#e67e80"), Amber: hex("#e69875"), Cyan: hex("#83c092"),
				OnColor: hex("#2d353b"),
			}
		}
		return Palette{
			Accent: hex("#8da101"), Text: hex("#5c6a72"), Muted: hex("#939f91"), Border: hex("#e6e2cc"),
			CursorBg: hex("#e6e2cc"), DimBg: hex("#f4f0d9"), ChipBg: hex("#efebd4"),
			Green: hex("#8da101"), Red: hex("#f85552"), Amber: hex("#f57d26"), Cyan: hex("#35a77c"),
			OnColor: hex("#fdf6e3"),
		}
	case "tokyonight":
		if dark { // Night
			return Palette{
				Accent: hex("#7aa2f7"), Text: hex("#c0caf5"), Muted: hex("#565f89"), Border: hex("#3b4261"),
				CursorBg: hex("#283457"), DimBg: hex("#292e42"), ChipBg: hex("#292e42"),
				Green: hex("#9ece6a"), Red: hex("#f7768e"), Amber: hex("#ff9e64"), Cyan: hex("#7dcfff"),
				OnColor: hex("#1a1b26"),
			}
		}
		return Palette{ // Day
			Accent: hex("#2e7de9"), Text: hex("#3760bf"), Muted: hex("#848cb5"), Border: hex("#c4c8da"),
			CursorBg: hex("#c4c8da"), DimBg: hex("#d0d5e3"), ChipBg: hex("#d0d5e3"),
			Green: hex("#587539"), Red: hex("#f52a65"), Amber: hex("#b15c00"), Cyan: hex("#007197"),
			OnColor: hex("#e1e2e7"),
		}
	case "dracula":
		return Palette{
			Accent: hex("#bd93f9"), Text: hex("#f8f8f2"), Muted: hex("#6272a4"), Border: hex("#44475a"),
			CursorBg: hex("#44475a"), DimBg: hex("#343746"), ChipBg: hex("#44475a"),
			Green: hex("#50fa7b"), Red: hex("#ff5555"), Amber: hex("#ffb86c"), Cyan: hex("#8be9fd"),
			OnColor: hex("#282a36"),
		}
	}
	// lazyc7n, the default.
	ld := lipgloss.LightDark(dark)
	return Palette{
		Accent: ld(hex("#5A56E0"), hex("#7D79F6")), Text: ld(hex("#1C1C24"), hex("#E4E4EC")),
		Muted: ld(hex("#8A8A8A"), hex("#6C6C6C")), Border: ld(hex("#C8C8D0"), hex("#3E3E50")),
		CursorBg: ld(hex("#E2E1FB"), hex("#34325E")), DimBg: ld(hex("#ECECF0"), hex("#26263A")),
		ChipBg: ld(hex("#ECECF0"), hex("#2A2A3C")),
		Green:  ld(hex("#1F8A55"), hex("#4CC38A")), Red: hex("#E5484D"), Amber: ld(hex("#B26A00"), hex("#F5A524")),
		Cyan: ld(hex("#0B7A8C"), hex("#5FC9DB")), OnColor: hex("#101014"),
	}
}

// NewStyles builds every style from a theme.
func NewStyles(theme string, dark bool) Styles {
	p := PaletteFor(theme, dark)
	pill := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(bg).Foreground(fg)
	}
	pane := func(c color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 1)
	}
	cursor := lipgloss.NewStyle().Bold(true).Reverse(true)
	if p.CursorBg != nil {
		cursor = lipgloss.NewStyle().Bold(true).Background(p.CursorBg).Foreground(p.Text)
	}
	dim := lipgloss.NewStyle().Underline(true)
	if p.DimBg != nil {
		dim = lipgloss.NewStyle().Background(p.DimBg).Foreground(p.Text)
	}

	return Styles{
		Dark:        dark,
		Brand:       pill(p.Accent, p.OnColor),
		Tab:         lipgloss.NewStyle().Padding(0, 1).Foreground(p.Muted),
		ActiveTab:   lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(p.Accent).Underline(true),
		Pane:        pane(p.Border),
		FocusedPane: pane(p.Accent),
		PaneTitle:   lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Item:        lipgloss.NewStyle().Foreground(p.Text),
		Bold:        lipgloss.NewStyle().Bold(true).Foreground(p.Text),
		Muted:       lipgloss.NewStyle().Foreground(p.Muted),
		Cursor:      cursor,
		CursorDim:   dim,
		Marked:      lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		DryBadge:    pill(p.Green, p.OnColor),
		LiveBadge:   pill(p.Red, p.OnColor),
		Status:      lipgloss.NewStyle().Foreground(p.Muted),
		StatusError: lipgloss.NewStyle().Bold(true).Foreground(p.Red),
		OK:          lipgloss.NewStyle().Foreground(p.Green),
		Chip:        lipgloss.NewStyle().Background(p.ChipBg).Foreground(p.Text).Padding(0, 1),

		Key:     lipgloss.NewStyle().Foreground(p.Accent),
		String:  lipgloss.NewStyle().Foreground(p.Green),
		Number:  lipgloss.NewStyle().Foreground(p.Cyan),
		Comment: lipgloss.NewStyle().Foreground(p.Muted).Italic(true),

		GatePane: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(p.Red).
			Padding(0, 1),
		Danger: lipgloss.NewStyle().Bold(true).Foreground(p.Red),
		Warn:   lipgloss.NewStyle().Foreground(p.Amber),
		Input:  lipgloss.NewStyle().Bold(true).Foreground(p.Text),
	}
}
