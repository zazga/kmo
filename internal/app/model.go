package app

import (
	"context"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/zazga/kmo/internal/config"
	"github.com/zazga/kmo/internal/daemon"
	"github.com/zazga/kmo/internal/keychain"
	mm "github.com/zazga/kmo/internal/mattermost"
	"github.com/zazga/kmo/internal/ui"
)

type mode int
const ( modeSetup mode = iota; modeConnecting; modeMain )

type Model struct {
	ctx context.Context
	cancel context.CancelFunc
	mode mode
	cfg config.Config
	keys keymap
	keychain keychain.Store
	service *mm.Service
	infoLog *slog.Logger
	errLog *slog.Logger
	user *model.User
	team *model.Team
	users map[string]*model.User
	conversations []*mm.Conversation
	active *mm.Conversation
	sidebar ui.Sidebar
	chat ui.Chat
	compose ui.Compose
	setup ui.Setup
	focus ui.Focus
	connection string
	telegramConnection string
	daemonStatuses <-chan daemon.Status
	showHelp bool
	width int
	height int
	statusErr string
	isDark bool
}

func New(cfg config.Config, token string, keyStore keychain.Store, infoLog, errLog *slog.Logger) Model {
	ctx, cancel := context.WithCancel(context.Background())
	statuses := daemon.WatchStatus(ctx)
	telegramStatus := "disabled"
	if cfg.Telegram.Enabled { telegramStatus = "connecting" }
	m := Model{ctx:ctx, cancel:cancel, cfg:cfg, keys:newKeymap(cfg.Keybindings), keychain:keyStore, infoLog:infoLog, errLog:errLog, sidebar:ui.NewSidebar(), chat:ui.NewChat(), compose:ui.NewCompose(), focus:ui.FocusSidebar, connection:"offline", telegramConnection:telegramStatus, daemonStatuses:statuses, users:make(map[string]*model.User)}
	telegramToken, _ := keychain.Telegram().Get()
	if cfg.Complete() && strings.TrimSpace(token) != "" { m.service = mm.New(cfg.ServerURL, token, infoLog, errLog); m.mode = modeConnecting } else { m.mode = modeSetup; m.setup = ui.NewSetup(cfg.ServerURL, token, cfg.Telegram.Enabled, telegramToken) }
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, waitDaemonStatusCmd(m.daemonStatuses)}
	if m.mode == modeConnecting && m.service != nil { cmds = append(cmds, bootstrapCmd(m.ctx, m.service)) }
	return tea.Batch(cmds...)
}

func (m Model) Close() { m.cancel(); if m.service != nil { m.service.Close() } }

func bootstrapCmd(ctx context.Context, svc *mm.Service) tea.Cmd { return func() tea.Msg { session, err := svc.Bootstrap(ctx); return bootstrapMsg{session:session, err:err} } }
func waitEventCmd(svc *mm.Service) tea.Cmd {
	if svc == nil { return nil }
	return func() tea.Msg { ev, ok := <-svc.Events(); if !ok { return websocketMsg{event:mm.Event{Kind:mm.EventOffline}} }; return websocketMsg{event:ev} }
}
func waitDaemonStatusCmd(ch <-chan daemon.Status) tea.Cmd {
	if ch == nil { return nil }
	return func() tea.Msg { status, ok := <-ch; if !ok { return daemonStatusMsg{status: daemon.Status{Daemon:"unavailable", Telegram:"unavailable"}} }; return daemonStatusMsg{status:status} }
}
