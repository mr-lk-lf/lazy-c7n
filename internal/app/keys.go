package app

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up, Down, Top, Bottom, PageUp, PageDown key.Binding
	Left, Right                             key.Binding // switch pane
	NextScreen, PrevScreen, Screens         key.Binding
	Enter, Back, Filter, Mark, Toggle       key.Binding
	Reload, Validate, DryRun, LiveRun       key.Binding
	Edit, Copy, Cancel, Theme               key.Binding
	Help, Quit                              key.Binding
}

func defaultKeys() keyMap {
	b := func(keys []string, helpKey, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, desc))
	}
	return keyMap{
		Up:         b([]string{"k", "up"}, "↑/k", "up"),
		Down:       b([]string{"j", "down"}, "↓/j", "down"),
		Top:        b([]string{"g", "home"}, "g", "top"),
		Bottom:     b([]string{"G", "end"}, "G", "bottom"),
		PageUp:     b([]string{"pgup", "ctrl+b"}, "pgup", "page up"),
		PageDown:   b([]string{"pgdown", "ctrl+f"}, "pgdn", "page down"),
		Left:       b([]string{"h", "left"}, "h/l", "pane"),
		Right:      b([]string{"l", "right"}, "l", "right pane"),
		NextScreen: b([]string{"tab"}, "tab", "next screen"),
		PrevScreen: b([]string{"shift+tab"}, "shift+tab", "prev screen"),
		Screens:    b([]string{"1", "2", "3", "4", "5"}, "1-5", "screen"),
		Enter:      b([]string{"enter"}, "enter", "open"),
		Back:       b([]string{"esc"}, "esc", "back"),
		Filter:     b([]string{"/"}, "/", "filter"),
		Mark:       b([]string{"space"}, "space", "select"),
		Toggle:     b([]string{"t"}, "t", "toggle view"),
		Reload:     b([]string{"r"}, "r", "reload"),
		Validate:   b([]string{"v"}, "v", "validate"),
		DryRun:     b([]string{"d"}, "d", "dry-run"),
		LiveRun:    b([]string{"R", "shift+r"}, "R", "live run"),
		Edit:       b([]string{"e"}, "e", "edit"),
		Copy:       b([]string{"y"}, "y", "copy command"),
		Cancel:     b([]string{"x"}, "x", "cancel job"),
		Theme:      b([]string{"T"}, "T", "next theme"),
		Help:       b([]string{"?"}, "?", "more keys"),
		Quit:       b([]string{"q"}, "q", "quit"),
	}
}

// screenHelp adapts the key help to the active screen.
type screenHelp struct {
	k      keyMap
	screen Screen
}

func (h screenHelp) ShortHelp() []key.Binding {
	k := h.k
	switch h.screen {
	case ScreenPolicies:
		return []key.Binding{k.Mark, k.DryRun, k.LiveRun, k.Filter, k.NextScreen, k.Help, k.Quit}
	case ScreenRuns:
		return []key.Binding{k.Enter, k.Toggle, k.Copy, k.Filter, k.NextScreen, k.Help, k.Quit}
	case ScreenResources:
		return []key.Binding{k.Left, k.Filter, k.NextScreen, k.Help, k.Quit}
	case ScreenSchema:
		return []key.Binding{k.Enter, k.Back, k.Filter, k.NextScreen, k.Help, k.Quit}
	case ScreenJobs:
		return []key.Binding{k.Cancel, k.NextScreen, k.Help, k.Quit}
	}
	return []key.Binding{k.Help, k.Quit}
}

func (h screenHelp) FullHelp() [][]key.Binding {
	k := h.k
	nav := []key.Binding{k.Up, k.Down, k.Top, k.Bottom, k.PageUp, k.PageDown}
	panes := []key.Binding{k.Left, k.NextScreen, k.PrevScreen, k.Screens, k.Enter, k.Back, k.Filter}
	var actions []key.Binding
	switch h.screen {
	case ScreenPolicies:
		actions = []key.Binding{k.Mark, k.Validate, k.DryRun, k.LiveRun, k.Edit, k.Copy, k.Reload}
	case ScreenRuns:
		actions = []key.Binding{k.Toggle, k.Copy, k.Reload}
	case ScreenResources:
		actions = []key.Binding{k.Copy}
	case ScreenSchema:
		actions = []key.Binding{k.Reload}
	case ScreenJobs:
		actions = []key.Binding{k.Cancel}
	}
	return [][]key.Binding{nav, panes, actions, {k.Theme, k.Help, k.Quit}}
}
