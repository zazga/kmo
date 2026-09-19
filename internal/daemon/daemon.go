package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/zazga/kmo/internal/config"
	"github.com/zazga/kmo/internal/keychain"
	mm "github.com/zazga/kmo/internal/mattermost"
	tg "github.com/zazga/kmo/internal/telegram"
)

const (
	heartbeatInterval = 30 * time.Second
	onlineInterval    = 240 * time.Second
	cooldownInterval  = 5 * time.Minute
)

type Status struct {
	Daemon        string `json:"daemon"`
	Mattermost    string `json:"mattermost"`
	Telegram      string `json:"telegram"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type statusHub struct {
	mu      sync.RWMutex
	started time.Time
	mm      string
	tg      string
	clients map[net.Conn]struct{}
}

func Run(ctx context.Context, cfg config.Config, mattermostToken, telegramToken string, infoLog, errLog *slog.Logger) error {
	hub := &statusHub{started: time.Now(), mm: "connecting", tg: telegramInitialStatus(cfg), clients: map[net.Conn]struct{}{}}
	go hub.serve(ctx, errLog)
	go hub.heartbeat(ctx)

	if cfg.Complete() && strings.TrimSpace(mattermostToken) != "" {
		go alwaysOnlineLoop(ctx, cfg.ServerURL, mattermostToken, hub, infoLog, errLog)
	} else {
		hub.setMattermost("unavailable")
	}

	if cfg.Telegram.Enabled {
		if cfg.Telegram.ChatID == 0 || strings.TrimSpace(telegramToken) == "" {
			hub.setTelegram("unavailable")
		} else {
			go forwardingSupervisor(ctx, cfg, mattermostToken, telegramToken, hub, infoLog, errLog)
		}
	}

	<-ctx.Done()
	return nil
}

func telegramInitialStatus(cfg config.Config) string {
	if !cfg.Telegram.Enabled {
		return "disabled"
	}
	return "connecting"
}

func alwaysOnlineLoop(ctx context.Context, serverURL, token string, hub *statusHub, infoLog, errLog *slog.Logger) {
	svc := mm.New(serverURL, token, infoLog, errLog)
	defer svc.Close()
	ticker := time.NewTicker(onlineInterval)
	defer ticker.Stop()
	for {
		if err := retryOperation(ctx, hub.setMattermost, func() error { return svc.SetOnline(ctx) }); err == nil {
			hub.setMattermost("connected")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func forwardingSupervisor(ctx context.Context, cfg config.Config, mmToken, tgToken string, hub *statusHub, infoLog, errLog *slog.Logger) {
	for {
		if ctx.Err() != nil {
			return
		}
		hub.setTelegram("reconnecting")
		if err := runForwardingSession(ctx, cfg, mmToken, tgToken, hub, infoLog, errLog); err != nil && errLog != nil {
			errLog.Warn("forwarding session ended", "component", "daemon", "error", err)
		}
		hub.setTelegram("unavailable")
		if !sleepContext(ctx, cooldownInterval) {
			return
		}
	}
}

func runForwardingSession(ctx context.Context, cfg config.Config, mmToken, tgToken string, hub *statusHub, infoLog, errLog *slog.Logger) error {
	telegram := tg.New(tgToken)
	if _, err := telegram.GetMe(ctx); err != nil {
		if isTelegramAuthError(err) {
			disableTelegram(errLog)
			hub.setTelegram("disabled")
			return fmt.Errorf("Telegram token rejected: %w", err)
		}
		return err
	}
	state, err := tg.LoadState()
	if err != nil {
		return err
	}
	mattermost := mm.New(cfg.ServerURL, mmToken, infoLog, errLog)
	defer mattermost.Close()
	session, err := mattermost.Bootstrap(ctx)
	if err != nil {
		return err
	}
	convs := map[string]*mm.Conversation{}
	for _, c := range session.Conversations {
		if c != nil && c.Channel != nil {
			convs[c.Channel.Id] = c
		}
	}
	mattermost.StartWebSocket(ctx)
	hub.setTelegram("connected")

	updates := make(chan tg.Update, 32)
	errs := make(chan error, 2)
	go telegramPollLoop(ctx, telegram, updates, errs)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errs:
			return err
		case up := <-updates:
			handleTelegramReply(ctx, telegram, mattermost, state, cfg.Telegram.ChatID, up, errLog)
		case ev := <-mattermost.Events():
			switch ev.Kind {
			case mm.EventOffline:
				if ev.Err != nil {
					return ev.Err
				}
			case mm.EventRefreshConversations:
				items, loadErr := mattermost.LoadConversations(ctx, session.User, session.Team)
				if loadErr == nil {
					convs = map[string]*mm.Conversation{}
					for _, c := range items {
						if c != nil && c.Channel != nil { convs[c.Channel.Id] = c }
					}
				}
			case mm.EventPosted:
				if ev.Post == nil || ev.Post.UserId == session.User.Id || strings.TrimSpace(ev.Post.Message) == "" {
					continue
				}
				conv := convs[ev.Post.ChannelId]
				if conv == nil || conv.Kind != mm.ConversationDM {
					continue
				}
				header := fmt.Sprintf("%s · %s", conv.Display, time.UnixMilli(ev.Post.CreateAt).Local().Format("15:04"))
				if ev.Post.RootId != "" {
					header += " · reply"
				}
				for _, part := range tg.SplitMessage(header, ev.Post.Message) {
					msg, sendErr := telegram.SendMessage(ctx, cfg.Telegram.ChatID, part)
					if sendErr != nil {
						return sendErr
					}
					if err := state.Put(msg.MessageID, ev.Post.ChannelId, time.Now()); err != nil && errLog != nil {
						errLog.Warn("persist Telegram mapping failed", "component", "telegram", "error", err)
					}
				}
			}
		}
	}
}

func telegramPollLoop(ctx context.Context, client *tg.Client, out chan<- tg.Update, errs chan<- error) {
	var offset int64
	for {
		updates, err := client.GetUpdates(ctx, offset, 50)
		if err != nil {
			select { case errs <- err: default: }
			return
		}
		for _, up := range updates {
			if up.UpdateID >= offset { offset = up.UpdateID + 1 }
			select { case out <- up: case <-ctx.Done(): return }
		}
		if ctx.Err() != nil { return }
	}
}

func handleTelegramReply(ctx context.Context, client *tg.Client, mattermost *mm.Service, state *tg.State, chatID int64, up tg.Update, errLog *slog.Logger) {
	msg := up.Message
	if msg == nil || msg.Chat.ID != chatID || strings.TrimSpace(msg.Text) == "" || msg.ReplyToMessage == nil {
		return
	}
	channelID, ok, expired := state.Resolve(msg.ReplyToMessage.MessageID, time.Now())
	if expired {
		_, _ = client.SendMessage(ctx, chatID, "Deze reply is verlopen.")
		return
	}
	if !ok {
		return
	}
	if _, err := mattermost.SendPost(ctx, channelID, msg.Text, ""); err != nil {
		_, _ = client.SendMessage(ctx, chatID, "Kon bericht niet naar Mattermost sturen: "+err.Error())
		if errLog != nil { errLog.Warn("Telegram reply send failed", "component", "telegram", "error", err) }
	}
}

func retryOperation(ctx context.Context, setStatus func(string), op func() error) error {
	delays := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	for _, delay := range delays {
		setStatus("reconnecting")
		if err := op(); err == nil { return nil }
		if !sleepContext(ctx, delay) { return ctx.Err() }
	}
	setStatus("unavailable")
	for {
		if !sleepContext(ctx, cooldownInterval) { return ctx.Err() }
		setStatus("reconnecting")
		if err := op(); err == nil { return nil }
		setStatus("unavailable")
	}
}

func disableTelegram(errLog *slog.Logger) {
	cfg, _, err := config.Load()
	if err != nil { return }
	cfg.Telegram.Enabled = false
	if err := config.Save(cfg); err != nil && errLog != nil {
		errLog.Error("disable Telegram after auth failure", "component", "telegram", "error", err)
	}
}

func isTelegramAuthError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "401") || strings.Contains(s, "unauthorized")
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select { case <-ctx.Done(): return false; case <-t.C: return true }
}

func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil { return "", err }
	return filepath.Join(home, ".local", "state", "kmo", "kmo.sock"), nil
}

func (h *statusHub) snapshot() Status {
	h.mu.RLock(); defer h.mu.RUnlock()
	return Status{Daemon: "running", Mattermost: h.mm, Telegram: h.tg, UptimeSeconds: int64(time.Since(h.started).Seconds())}
}

func (h *statusHub) setMattermost(v string) { h.mu.Lock(); h.mm = v; h.mu.Unlock(); h.broadcast() }
func (h *statusHub) setTelegram(v string) { h.mu.Lock(); h.tg = v; h.mu.Unlock(); h.broadcast() }

func (h *statusHub) heartbeat(ctx context.Context) {
	t := time.NewTicker(heartbeatInterval); defer t.Stop()
	for { select { case <-ctx.Done(): return; case <-t.C: h.broadcast() } }
}

func (h *statusHub) serve(ctx context.Context, errLog *slog.Logger) {
	path, err := SocketPath(); if err != nil { return }
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil { if errLog != nil { errLog.Error("status socket listen failed", "component", "daemon", "error", err) }; return }
	defer func(){ ln.Close(); os.Remove(path) }()
	go func(){ <-ctx.Done(); ln.Close() }()
	for {
		conn, err := ln.Accept(); if err != nil { return }
		h.mu.Lock(); h.clients[conn] = struct{}{}; h.mu.Unlock()
		h.write(conn)
	}
}

func (h *statusHub) broadcast() {
	h.mu.RLock(); clients := make([]net.Conn,0,len(h.clients)); for c := range h.clients { clients=append(clients,c) }; h.mu.RUnlock()
	for _, c := range clients { h.write(c) }
}

func (h *statusHub) write(c net.Conn) {
	b, _ := json.Marshal(h.snapshot()); b = append(b, '\n')
	_ = c.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(b); err != nil { h.mu.Lock(); delete(h.clients,c); h.mu.Unlock(); c.Close() }
}

var _ = model.StatusOnline
var _ = keychain.Service
