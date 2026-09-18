package app

import (
	"testing"
	tea "charm.land/bubbletea/v2"
)

func TestCommandBindingUsesSuper(t *testing.T) { if !keyMatches(tea.KeyPressMsg(tea.Key{Code:'f', Mod:tea.ModSuper}), "cmd+f") { t.Fatal("expected super+f to match cmd+f") } }
func TestCommandBindingPortableFallback(t *testing.T) { if !keyMatches(tea.KeyPressMsg(tea.Key{Code:'f', Mod:tea.ModCtrl}), "cmd+f") { t.Fatal("expected ctrl+f fallback") } }
func TestCommandTabPortableFallback(t *testing.T) { if !keyMatches(tea.KeyPressMsg(tea.Key{Code:tea.KeyTab, Mod:tea.ModShift}), "cmd+tab") { t.Fatal("expected shift+tab fallback") } }
