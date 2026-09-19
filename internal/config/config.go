package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultConfigDir = ".config/kmo"
	defaultFileName  = "config.toml"
)

type Keybindings struct {
	FocusNext     string `toml:"focus_next"`
	FocusPrevious string `toml:"focus_previous"`
	Find          string `toml:"find"`
	Reply         string `toml:"reply"`
	Send          string `toml:"send"`
	Shortcuts     string `toml:"shortcuts"`
	Quit          string `toml:"quit"`
	Cancel        string `toml:"cancel"`
	Up            string `toml:"up"`
	Down          string `toml:"down"`
}

type Telegram struct {
	Enabled bool  `toml:"enabled"`
	ChatID  int64 `toml:"chat_id,omitempty"`
}

type Config struct {
	ServerURL          string      `toml:"server_url"`
	LastConversationID string      `toml:"last_conversation_id,omitempty"`
	Telegram           Telegram    `toml:"telegram"`
	Keybindings        Keybindings `toml:"keybindings"`
}

func Defaults() Config {
	return Config{Keybindings: Keybindings{
		FocusNext: "tab", FocusPrevious: "ctrl+tab", Find: "ctrl+f",
		Reply: "ctrl+r", Send: "ctrl+enter", Shortcuts: "ctrl+h", Quit: "ctrl+q",
		Cancel: "esc", Up: "up", Down: "down",
	}}
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultConfigDir, defaultFileName), nil
}

func Load() (Config, []error, error) {
	path, err := Path()
	if err != nil {
		return Config{}, nil, err
	}
	return LoadFrom(path)
}

func LoadFrom(path string) (Config, []error, error) {
	cfg := Defaults()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil, nil
		}
		return Config{}, nil, fmt.Errorf("stat config: %w", err)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("decode config: %w", err)
	}

	var warnings []error
	defaults := Defaults().Keybindings
	validate := func(name string, value *string, fallback string) {
		if !ValidBinding(*value) {
			warnings = append(warnings, fmt.Errorf("invalid keybinding %s=%q; using %q", name, *value, fallback))
			*value = fallback
		}
	}
	validate("focus_next", &cfg.Keybindings.FocusNext, defaults.FocusNext)
	validate("focus_previous", &cfg.Keybindings.FocusPrevious, defaults.FocusPrevious)
	validate("find", &cfg.Keybindings.Find, defaults.Find)
	validate("reply", &cfg.Keybindings.Reply, defaults.Reply)
	validate("send", &cfg.Keybindings.Send, defaults.Send)
	validate("shortcuts", &cfg.Keybindings.Shortcuts, defaults.Shortcuts)
	validate("quit", &cfg.Keybindings.Quit, defaults.Quit)
	validate("cancel", &cfg.Keybindings.Cancel, defaults.Cancel)
	validate("up", &cfg.Keybindings.Up, defaults.Up)
	validate("down", &cfg.Keybindings.Down, defaults.Down)

	seen := map[string]string{}
	type bindingRef struct {
		name     string
		value    *string
		fallback string
	}
	bindings := []bindingRef{
		{"focus_next", &cfg.Keybindings.FocusNext, defaults.FocusNext},
		{"focus_previous", &cfg.Keybindings.FocusPrevious, defaults.FocusPrevious},
		{"find", &cfg.Keybindings.Find, defaults.Find},
		{"reply", &cfg.Keybindings.Reply, defaults.Reply},
		{"send", &cfg.Keybindings.Send, defaults.Send},
		{"shortcuts", &cfg.Keybindings.Shortcuts, defaults.Shortcuts},
		{"quit", &cfg.Keybindings.Quit, defaults.Quit},
		{"cancel", &cfg.Keybindings.Cancel, defaults.Cancel},
		{"up", &cfg.Keybindings.Up, defaults.Up},
		{"down", &cfg.Keybindings.Down, defaults.Down},
	}
	for _, binding := range bindings {
		key := bindingIdentity(*binding.value)
		if other, ok := seen[key]; ok {
			warnings = append(warnings, fmt.Errorf("duplicate keybinding %s=%q conflicts with %s; using default %q", binding.name, *binding.value, other, binding.fallback))
			*binding.value = binding.fallback
			key = bindingIdentity(binding.fallback)
		}
		seen[key] = binding.name
	}
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	return cfg, warnings, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveTo(path, cfg)
}

func SaveTo(path string, cfg Config) error {
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

func (c Config) Complete() bool {
	return strings.HasPrefix(c.ServerURL, "http://") || strings.HasPrefix(c.ServerURL, "https://")
}

func ValidBinding(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	parts := strings.Split(s, "+")
	base := parts[len(parts)-1]
	if base == "" {
		return false
	}
	seenMods := map[string]bool{}
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "cmd", "super", "ctrl", "alt", "shift", "meta", "hyper":
			if seenMods[mod] {
				return false
			}
			seenMods[mod] = true
		default:
			return false
		}
	}
	named := map[string]bool{
		"tab": true, "esc": true, "escape": true, "enter": true, "return": true,
		"space": true, "backspace": true, "delete": true,
		"up": true, "down": true, "left": true, "right": true,
		"home": true, "end": true, "pageup": true, "pagedown": true,
		"pgup": true, "pgdown": true,
	}
	if named[base] {
		return true
	}
	return len([]rune(base)) == 1
}

func bindingIdentity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "cmd+", "super+")
	switch s {
	case "esc":
		return "escape"
	case "pgup":
		return "pageup"
	case "pgdown":
		return "pagedown"
	}
	return s
}
