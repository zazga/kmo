package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/zazga/kmo/internal/app"
	"github.com/zazga/kmo/internal/config"
	"github.com/zazga/kmo/internal/daemon"
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

	cfg, warnings, err := config.Load()
	if err != nil {
		logs.Error.Error("config load failed", "component", "config", "error", err)
		fmt.Fprintln(os.Stderr, "kmo: unable to load config:", err)
		os.Exit(1)
	}
	for _, warning := range warnings {
		logs.Error.Warn("config fallback", "component", "config", "error", warning)
	}

	mattermostStore := keychain.Default()
	mattermostToken, tokenErr := mattermostStore.Get()
	if tokenErr != nil {
		logs.Error.Warn("PAT not available at startup", "component", "keychain", "error", tokenErr)
		mattermostToken = ""
	}

	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		telegramToken, _ := keychain.Telegram().Get()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		logs.Info.Info("kmo daemon starting", "component", "daemon")
		if err := daemon.Run(ctx, cfg, mattermostToken, telegramToken, logs.Info, logs.Error); err != nil && ctx.Err() == nil {
			logs.Error.Error("daemon terminated", "component", "daemon", "error", err)
			os.Exit(1)
		}
		return
	}

	logs.Info.Info("kmo starting", "component", "main")
	model := app.New(cfg, mattermostToken, mattermostStore, logs.Info, logs.Error)
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
