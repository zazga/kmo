package app

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/zazga/kmo/internal/config"
)

type keymap struct {
	FocusNext     string
	FocusPrevious string
	Find          string
	Reply         string
	Send          string
	Shortcuts     string
	Cancel        string
	Up            string
	Down          string
}

func newKeymap(cfg config.Keybindings) keymap {
	return keymap{FocusNext: cfg.FocusNext, FocusPrevious: cfg.FocusPrevious, Find: cfg.Find, Reply: cfg.Reply, Send: cfg.Send, Shortcuts: cfg.Shortcuts, Cancel: cfg.Cancel, Up: cfg.Up, Down: cfg.Down}
}

func keyMatches(msg tea.KeyPressMsg, binding string) bool {
	got := normalizeKeystroke(msg.Keystroke())
	want := normalizeBinding(binding)
	if got == want {
		return true
	}
	if strings.HasPrefix(want, "super+") {
		rest := strings.TrimPrefix(want, "super+")
		if got == "ctrl+"+rest {
			return true
		}
		if rest == "tab" && got == "shift+tab" {
			return true
		}
	}
	return false
}

func normalizeBinding(binding string) string {
	binding = strings.ToLower(strings.TrimSpace(binding))
	binding = strings.ReplaceAll(binding, "cmd+", "super+")
	return normalizeKeystroke(binding)
}

func normalizeKeystroke(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "esc":
		return "escape"
	case "pgup":
		return "pageup"
	case "pgdown":
		return "pagedown"
	}

	parts := strings.Split(key, "+")
	if len(parts) == 1 {
		return key
	}
	base := parts[len(parts)-1]
	mods := append([]string(nil), parts[:len(parts)-1]...)

	// Terminals commonly report '?' as Shift+/ and may order modifiers
	// differently (super+shift+/ vs shift+super+/). Canonicalize both forms.
	if base == "/" && containsString(mods, "shift") {
		base = "?"
		mods = removeString(mods, "shift")
	}
	for i, mod := range mods {
		if mod == "cmd" {
			mods[i] = "super"
		}
	}
	sort.Strings(mods)
	if len(mods) == 0 {
		return base
	}
	return strings.Join(append(mods, base), "+")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
