package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zazga/kmo/internal/config"
	mm "github.com/zazga/kmo/internal/mattermost"
	tg "github.com/zazga/kmo/internal/telegram"
)

const (
	heartbeatInterval = 30 * time.Second
	onlineInterval    = 240 * time.Second
	cooldownInterval  = 5 * time.Minute
)

var errTelegramDisabled = errors.New("telegram forwarding disabled")

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
		if cfg.Telegram.ChatID == 0 || strings.TrimSpace(telegramToken) == "" || strings.TrimSpace(mattermostToken) == "" || !cfg.Complete() {
			hub.setTelegram("unavailable")
		} else {
			state, err := tg.LoadState()
			if err != nil {
				return err
			}
			client := tg.New(telegramToken)
			go telegramReplyLoop(ctx, cfg, mattermostToken, client, state, hub, infoLog, errLog)
			go mattermostForwardLoop(ctx, cfg, mattermostToken, client, state, hub, infoLog, errLog)
			go stateCleanupLoop(ctx, state, errLog)
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
	if err := retryOperation(ctx, hub.setMattermost, func() error { return svc.SetOnline(ctx) }); err == nil {
		hub.setMattermost("connected")
	}
	ticker := time.NewTicker(onlineInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := retryOperation(ctx, hub.setMattermost, func() error { return svc.SetOnline(ctx) }); err == nil {
				hub.setMattermost("connected")
			}
		}
	}
}

func telegramReplyLoop(ctx context.Context, cfg config.Config, mmToken string, client *tg.Client, state *tg.State, hub *statusHub, infoLog, errLog *slog.Logger) {
	replySvc := mm.New(cfg.ServerURL, mmToken, infoLog, errLog)
	defer replySvc.Close()
	delays := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	attempt := 0
	for ctx.Err() == nil {
		hub.setTelegram("reconnecting")
		err := runTelegramPollSession(ctx, cfg.Telegram.ChatID, client, replySvc, state, hub, errLog)
		if ctx.Err() != nil {
			return
		}
		if err != nil && isTelegramAuthError(err) {
			disableTelegram(errLog)
			hub.setTelegram("disabled")
			if errLog != nil {
				errLog.Error("Telegram token rejected; forwarding disabled", "component", "telegram", "error", err)
			}
			return
		}
		if err != nil && errLog != nil {
			errLog.Warn("Telegram polling disconnected", "component", "telegram", "error", err)
		}
		if attempt < len(delays) {
			d := delays[attempt]
			attempt++
			if !sleepContext(ctx, d) {
				return
			}
			continue
		}
		hub.setTelegram("unavailable")
		if !sleepContext(ctx, cooldownInterval) {
			return
		}
		attempt = 0
	}
}

func runTelegramPollSession(ctx context.Context, chatID int64, client *tg.Client, replySvc *mm.Service, state *tg.State, hub *statusHub, errLog *slog.Logger) error {
	if _, err := client.GetMe(ctx); err != nil {
		return err
	}
	hub.setTelegram("connected")
	for ctx.Err() == nil {
		updates, err := client.GetUpdates(ctx, state.Offset(), 50)
		if err != nil {
			return err
		}
		for _, up := range updates {
			handleTelegramReply(ctx, client, replySvc, state, chatID, up, errLog)
			if err := state.AdvanceUpdate(up.UpdateID); err != nil && errLog != nil {
				errLog.Warn("persist Telegram update offset failed", "component", "telegram", "error", err)
			}
		}
	}
	return ctx.Err()
}

func mattermostForwardLoop(ctx context.Context, cfg config.Config, mmToken string, client *tg.Client, state *tg.State, hub *statusHub, infoLog, errLog *slog.Logger) {
	delays := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	attempt := 0
	for ctx.Err() == nil {
		err := runMattermostForwardSession(ctx, cfg, mmToken, client, state, hub, infoLog, errLog)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errTelegramDisabled) {
			return
		}
		if err != nil && errLog != nil {
			errLog.Warn("Mattermost forwarding disconnected", "component", "mattermost", "error", err)
		}
		if attempt < len(delays) {
			d := delays[attempt]
			attempt++
			if !sleepContext(ctx, d) {
				return
			}
			continue
		}
		if !sleepContext(ctx, cooldownInterval) {
			return
		}
		attempt = 0
	}
}

func runMattermostForwardSession(ctx context.Context, cfg config.Config, mmToken string, client *tg.Client, state *tg.State, hub *statusHub, infoLog, errLog *slog.Logger) error {
	svc := mm.New(cfg.ServerURL, mmToken, infoLog, errLog)
	defer svc.Close()
	session, err := svc.Bootstrap(ctx)
	if err != nil {
		return err
	}
	convs := map[string]*mm.Conversation{}
	for _, c := range session.Conversations {
		if c != nil && c.Channel != nil {
			convs[c.Channel.Id] = c
		}
	}
	svc.StartWebSocket(ctx)
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-svc.Events():
			switch ev.Kind {
			case mm.EventRefreshConversations:
				items, loadErr := svc.LoadConversations(ctx, session.User, session.Team)
				if loadErr != nil {
					if errLog != nil {
						errLog.Warn("refresh daemon conversations failed", "component", "mattermost", "error", loadErr)
					}
					continue
				}
				convs = map[string]*mm.Conversation{}
				for _, c := range items {
					if c != nil && c.Channel != nil {
						convs[c.Channel.Id] = c
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
					msg, sendErr := client.SendMessage(ctx, cfg.Telegram.ChatID, part)
					if sendErr != nil {
						if isTelegramAuthError(sendErr) {
							disableTelegram(errLog)
							hub.setTelegram("disabled")
							return errTelegramDisabled
						}
						hub.setTelegram("reconnecting")
						if errLog != nil {
							errLog.Warn("forward DM to Telegram failed", "component", "telegram", "error", sendErr)
						}
						continue
					}
					if err := state.Put(msg.MessageID, ev.Post.ChannelId, time.Now()); err != nil && errLog != nil {
						errLog.Warn("persist Telegram mapping failed", "component", "telegram", "error", err)
					}
				}
			}
		}
	}
	return ctx.Err()
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
		if errLog != nil {
			errLog.Warn("Telegram reply send failed", "component", "telegram", "error", err)
		}
	}
}

func stateCleanupLoop(ctx context.Context, state *tg.State, errLog *slog.Logger) {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := state.Cleanup(time.Now()); err != nil && errLog != nil {
				errLog.Warn("Telegram state cleanup failed", "component", "telegram", "error", err)
			}
		}
	}
}

func retryOperation(ctx context.Context, setStatus func(string), op func() error) error {
	delays := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	for _, delay := range delays {
		setStatus("reconnecting")
		if err := op(); err == nil {
			return nil
		}
		if !sleepContext(ctx, delay) {
			return ctx.Err()
		}
	}
	setStatus("unavailable")
	for {
		if !sleepContext(ctx, cooldownInterval) {
			return ctx.Err()
		}
		setStatus("reconnecting")
		if err := op(); err == nil {
			return nil
		}
		setStatus("unavailable")
	}
}

func disableTelegram(errLog *slog.Logger) {
	cfg, _, err := config.Load()
	if err != nil {
		if errLog != nil {
			errLog.Error("load config while disabling Telegram", "component", "telegram", "error", err)
		}
		return
	}
	cfg.Telegram.Enabled = false
	if err := config.Save(cfg); err != nil && errLog != nil {
		errLog.Error("disable Telegram after auth failure", "component", "telegram", "error", err)
	}
}

func isTelegramAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "401") || strings.Contains(s, "unauthorized")
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "kmo", "kmo.sock"), nil
}

func (h *statusHub) snapshot() Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return Status{Daemon: "running", Mattermost: h.mm, Telegram: h.tg, UptimeSeconds: int64(time.Since(h.started).Seconds())}
}

func (h *statusHub) setMattermost(v string) {
	h.mu.Lock()
	h.mm = v
	h.mu.Unlock()
	h.broadcast()
}

func (h *statusHub) setTelegram(v string) {
	h.mu.Lock()
	h.tg = v
	h.mu.Unlock()
	h.broadcast()
}

func (h *statusHub) heartbeat(ctx context.Context) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.broadcast()
		}
	}
}

func (h *statusHub) serve(ctx context.Context, errLog *slog.Logger) {
	path, err := SocketPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		if errLog != nil {
			errLog.Error("status socket listen failed", "component", "daemon", "error", err)
		}
		return
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(path)
	}()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		h.mu.Lock()
		h.clients[conn] = struct{}{}
		h.mu.Unlock()
		h.write(conn)
	}
}

func (h *statusHub) broadcast() {
	h.mu.RLock()
	clients := make([]net.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		h.write(c)
	}
}

func (h *statusHub) write(c net.Conn) {
	b, _ := json.Marshal(h.snapshot())
	b = append(b, '\n')
	_ = c.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(b); err != nil {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		_ = c.Close()
	}
}
