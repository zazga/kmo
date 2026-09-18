package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/zazga/kmo/internal/app"
	"github.com/zazga/kmo/internal/config"
	"github.com/zazga/kmo/internal/keychain"
	"github.com/zazga/kmo/internal/logging"
)

func main() {
	logs, err := logging.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kmo: unable to initialize logs:", err)
		os.Exit(1)
	}
	defer logs.Close()
	logs.Info.Info("kmo starting", "component", "main")

	cfg, warnings, err := config.Load()
	if err != nil {
		logs.Error.Error("config load failed", "component", "config", "error", err)
		fmt.Fprintln(os.Stderr, "kmo: unable to load config:", err)
		os.Exit(1)
	}
	for _, warning := range warnings {
		logs.Error.Warn("config fallback", "component", "config", "error", warning)
	}

	store := keychain.Default()
	token, tokenErr := store.Get()
	if tokenErr != nil {
		logs.Error.Warn("PAT not available at startup", "component", "keychain", "error", tokenErr)
		token = ""
	}

	model := app.New(cfg, token, store, logs.Info, logs.Error)
	program := tea.NewProgram(model)
	final, err := program.Run()
	if closer, ok := final.(interface{ Close() }); ok {
		closer.Close()
	}
	if err != nil {
		logs.Error.Error("TUI terminated with error", "component", "main", "error", err)
		fmt.Fprintln(os.Stderr, "kmo:", err)
		os.Exit(1)
	}
}
