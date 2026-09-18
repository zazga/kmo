package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingConfigUsesDefaults(t *testing.T) {
	cfg, warnings, err := LoadFrom(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if cfg.Keybindings.Send != "ctrl+enter" {
		t.Fatalf("unexpected send default: %q", cfg.Keybindings.Send)
	}
}

func TestInvalidBindingFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keybindings]\nsend = 'definitely-not-a-key'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Keybindings.Send != "ctrl+enter" {
		t.Fatalf("expected fallback, got %q", cfg.Keybindings.Send)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Defaults()
	cfg.ServerURL = "https://mattermost.example.com/"
	cfg.LastConversationID = "abc"
	if err := SaveTo(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServerURL != "https://mattermost.example.com" || got.LastConversationID != "abc" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestDuplicateBindingFallsBackDeterministically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "[keybindings]\nfind = 'ctrl+f'\nsend = 'ctrl+f'\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Keybindings.Find != "ctrl+f" {
		t.Fatalf("find changed unexpectedly: %q", cfg.Keybindings.Find)
	}
	if cfg.Keybindings.Send != Defaults().Keybindings.Send {
		t.Fatalf("send did not fall back: %q", cfg.Keybindings.Send)
	}
	if len(warnings) == 0 {
		t.Fatal("expected duplicate warning")
	}
}

func TestCtrlDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Keybindings.Find != "ctrl+f" || cfg.Keybindings.Send != "ctrl+enter" || cfg.Keybindings.Shortcuts != "ctrl+h" || cfg.Keybindings.Quit != "ctrl+q" {
		t.Fatalf("unexpected ctrl defaults: %#v", cfg.Keybindings)
	}
}
