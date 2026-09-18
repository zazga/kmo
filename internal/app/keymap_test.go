package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCommandBindingUsesSuper(t *testing.T) {
	if !keyMatches(tea.KeyPressMsg(tea.Key{Code: 'f', Mod: tea.ModSuper}), "cmd+f") {
		t.Fatal("expected super+f to match cmd+f")
	}
}

func TestCommandBindingPortableFallback(t *testing.T) {
	if !keyMatches(tea.KeyPressMsg(tea.Key{Code: 'f', Mod: tea.ModCtrl}), "cmd+f") {
		t.Fatal("expected ctrl+f fallback")
	}
}

func TestCommandTabPortableFallback(t *testing.T) {
	if !keyMatches(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}), "cmd+tab") {
		t.Fatal("expected shift+tab fallback")
	}
}

func TestCommandQuestionMarkMatchesShiftSlash(t *testing.T) {
	if !keyMatches(tea.KeyPressMsg(tea.Key{Code: '/', Mod: tea.ModSuper | tea.ModShift}), "cmd+?") {
		t.Fatal("expected super+shift+/ to match cmd+?")
	}
}

func TestCommandQuestionMarkPortableFallback(t *testing.T) {
	if !keyMatches(tea.KeyPressMsg(tea.Key{Code: '/', Mod: tea.ModCtrl | tea.ModShift}), "cmd+?") {
		t.Fatal("expected ctrl+shift+/ fallback to match cmd+?")
	}
}

func TestCommandQuestionMarkWithShiftedQuestionMark(t *testing.T) {
	msg := tea.KeyPressMsg(tea.Key{Code: '?', Mod: tea.ModSuper | tea.ModShift})
	if !keyMatches(msg, "cmd+?") {
		t.Fatal("expected cmd+shift+? terminal form to match cmd+?")
	}
}
