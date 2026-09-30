package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/config"
)

func press(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text}
}

func send(m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m, cmd
}

func plain(m Model) string { return ansi.Strip(m.View().Content) }

func TestStartsOnPolicies(t *testing.T) {
	if s := New(config.Default()).Screen(); s != ScreenPolicies {
		t.Fatalf("screen = %v", s)
	}
}

func TestScreensCycleBothWays(t *testing.T) {
	m, _ := send(New(config.Default()), tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.Screen() != ScreenJobs {
		t.Fatalf("shift+tab from first: %v", m.Screen())
	}
	for range Screens {
		m, _ = send(m, press('l', "l"))
	}
	if m.Screen() != ScreenJobs {
		t.Fatalf("full cycle: %v", m.Screen())
	}
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.Screen() != ScreenPolicies {
		t.Fatalf("wrap: %v", m.Screen())
	}
}

func TestQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyPressMsg{press('q', "q"), {Code: 'c', Mod: tea.ModCtrl}} {
		_, cmd := send(New(config.Default()), msg)
		if cmd == nil {
			t.Fatalf("%v: no command", msg)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%v: not a quit command", msg)
		}
	}
}

func TestOtherKeysDoNothing(t *testing.T) {
	m, cmd := send(New(config.Default()), press('x', "x"))
	if cmd != nil || m.Screen() != ScreenPolicies {
		t.Fatalf("unexpected effect: cmd=%v screen=%v", cmd, m.Screen())
	}
}

func TestPoliciesScreenShowsDirsAndDryBadge(t *testing.T) {
	m, _ := send(New(config.Default()), tea.WindowSizeMsg{Width: 90, Height: 12})
	out := plain(m)
	for _, want := range []string{"lazyc7n", "Policies", "./policies", "DRY", "q quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "LIVE") {
		t.Errorf("LIVE badge shown in dry-run mode:\n%s", out)
	}
}

func TestLiveDefaultIsVisible(t *testing.T) {
	cfg := config.Default()
	cfg.Safety.DefaultDryRun = false
	if out := plain(New(cfg)); !strings.Contains(out, "LIVE") {
		t.Fatalf("no LIVE badge:\n%s", out)
	}
}

func TestViewFitsWindow(t *testing.T) {
	m, _ := send(New(config.Default()), tea.WindowSizeMsg{Width: 70, Height: 10})
	lines := strings.Split(plain(m), "\n")
	if len(lines) > 10 {
		t.Errorf("height %d > 10", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 70 {
			t.Errorf("line %d width %d > 70: %q", i, w, l)
		}
	}
}
