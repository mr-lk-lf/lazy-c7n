package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

// isolateUserConfig points $XDG_CONFIG_HOME at an empty dir so a real user
// config on the dev machine cannot leak into tests.
func isolateUserConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	xdg.Reload()
	t.Cleanup(xdg.Reload)
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultIsSafe(t *testing.T) {
	cfg := Default()
	if !cfg.Safety.DefaultDryRun || cfg.Safety.ConfirmLive != ConfirmTypeName {
		t.Fatalf("unsafe default: %+v", cfg.Safety)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyFileKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(write(t, dir, "c.toml", ""), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("got %+v", cfg)
	}
}

func TestUnknownKeysAreIgnored(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(write(t, dir, "c.toml", "future_key = 1\n[runner]\nkind = \"docker\"\nx = 2\n"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runner.Kind != RunnerDocker || cfg.Runner.Custodian != "custodian" {
		t.Fatalf("got %+v", cfg.Runner)
	}
}

func TestProjectFileOverridesOnlyItsKeys(t *testing.T) {
	dir := t.TempDir()
	isolateUserConfig(t)
	write(t, dir, ProjectFile, "policy_dirs = [\"a\", \"b\"]\n[runner]\nkind = \"command\"\ncommand = [\"uvx\", \"custodian\"]\n")
	cfg, err := Load("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.PolicyDirs, []string{"a", "b"}) {
		t.Fatalf("policy_dirs = %v", cfg.PolicyDirs)
	}
	if cfg.Runner.Kind != RunnerCommand || cfg.Runner.Custodian != "custodian" || !cfg.Safety.DefaultDryRun {
		t.Fatalf("got %+v", cfg)
	}
}

func TestNoFilesIsDefault(t *testing.T) {
	isolateUserConfig(t)
	cfg, err := Load("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("got %+v", cfg)
	}
}

func TestUserConfigThenProjectFile(t *testing.T) {
	isolateUserConfig(t)
	if err := os.MkdirAll(filepath.Dir(UserConfigPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserConfigPath(), []byte("[runner]\ncustodian = \"/venv/bin/custodian\"\n[safety]\nconfirm_live = \"yes-no\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write(t, dir, ProjectFile, "[safety]\nconfirm_live = \"type-name\"\n")
	cfg, err := Load("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runner.Custodian != "/venv/bin/custodian" || cfg.Safety.ConfirmLive != ConfirmTypeName {
		t.Fatalf("got %+v", cfg)
	}
}

func TestExplicitMissingFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "nope.toml"), dir)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestInvalidSafetyValueFailsClosed(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(write(t, dir, "c.toml", "[safety]\nconfirm_live = \"nah\"\n"), dir)
	if err == nil || !strings.Contains(err.Error(), "safety.confirm_live") {
		t.Fatalf("err = %v", err)
	}
}

func TestInvalidTomlIsAnError(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(write(t, dir, "c.toml", "policy_dirs = ["), dir); err == nil {
		t.Fatal("expected parse error")
	}
}

// yes-no was rejected for v0 (PM decision, 2026-09-30): only typed
// confirmation. A config asking for it must not load at all.
func TestYesNoConfirmationIsRejected(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(write(t, dir, "c.toml", "[safety]\nconfirm_live = \"yes-no\"\n"), dir)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v", err)
	}

	isolateUserConfig(t)
	if err := os.MkdirAll(filepath.Dir(UserConfigPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserConfigPath(), []byte("[safety]\nconfirm_live = \"yes-no\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("", t.TempDir()); err == nil {
		t.Fatal("yes-no in the user config loaded")
	}
}

func TestEmptyConfirmValueFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(write(t, dir, "c.toml", "[safety]\nconfirm_live = \"\"\n"), dir); err == nil {
		t.Fatal("empty confirm_live loaded")
	}
}

func TestCommandRunnerNeedsCommand(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(write(t, dir, "c.toml", "[runner]\nkind = \"command\"\n"), dir)
	if err == nil || !strings.Contains(err.Error(), "runner.command") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatePath(t *testing.T) {
	cfg := Default()
	if !strings.HasSuffix(cfg.StatePath(), "lazyc7n") {
		t.Fatalf("default state path %q", cfg.StatePath())
	}
	cfg.StateDir = "/tmp/x"
	if cfg.StatePath() != "/tmp/x" {
		t.Fatalf("state path %q", cfg.StatePath())
	}
}

func TestThemeValues(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(write(t, dir, "c.toml", "theme = \"gruvbox\"\nappearance = \"light\"\n"), dir)
	if err != nil || cfg.Theme != "gruvbox" || cfg.Appearance != AppearanceLight {
		t.Fatalf("cfg=%v/%v err=%v", cfg.Theme, cfg.Appearance, err)
	}
	if _, err := Load(write(t, dir, "d.toml", "theme = \"pink\"\n"), dir); err == nil || !strings.Contains(err.Error(), "catppuccin") {
		t.Fatalf("unknown theme: %v", err)
	}
	if _, err := Load(write(t, dir, "e.toml", "appearance = \"dim\"\n"), dir); err == nil {
		t.Fatal("unknown appearance accepted")
	}
}
