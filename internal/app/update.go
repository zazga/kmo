package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/zazga/kmo/internal/config"
	"github.com/zazga/kmo/internal/keychain"
	"github.com/zazga/kmo/internal/launchd"
	mm "github.com/zazga/kmo/internal/mattermost"
	tg "github.com/zazga/kmo/internal/telegram"
	"github.com/zazga/kmo/internal/ui"
)

const (
	historyPageSize    = 60
	telegramPairWindow = 2 * time.Minute
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshChatContent(false)
		return m, nil

	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		ui.ApplyTheme(&m.sidebar, &m.compose, &m.setup, m.isDark)
		return m, nil

	case daemonStatusMsg:
		if msg.status.Telegram != "" {
			m.telegramConnection = msg.status.Telegram
		}
		return m, waitDaemonStatusCmd(m.daemonStatuses)

	case bootstrapMsg:
		if msg.err != nil {
			m.errLog.Error("bootstrap failed", "component", "mattermost", "error", msg.err)
			m.statusErr = msg.err.Error()
			m.mode = modeSetup
			token, _ := m.keychain.Get()
			telegramToken, _ := keychain.Telegram().Get()
			m.setup = ui.NewSetup(m.cfg.ServerURL, token, m.cfg.Telegram.Enabled, telegramToken)
			ui.ApplyTheme(&m.sidebar, &m.compose, &m.setup, m.isDark)
			return m, nil
		}
		return m.enterMain(msg.session)

	case setupValidatedMsg:
		m.setup.Busy = false
		if msg.err != nil {
			m.setup.Error = msg.err.Error()
			m.errLog.Warn("setup validation failed", "component", "setup", "error", msg.err)
			return m, nil
		}
		m.service = msg.service
		m.cfg = msg.cfg
		if m.cfg.Telegram.Enabled {
			m.telegramConnection = "connecting"
		} else {
			m.telegramConnection = "disabled"
		}
		return m.enterMain(msg.session)

	case messagesLoadedMsg:
		if msg.channelID != m.chat.ChannelID {
			return m, nil
		}
		m.chat.LoadingOlder = false
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			m.errLog.Warn("history load failed", "component", "mattermost", "channel_id", msg.channelID, "error", msg.err)
			return m, nil
		}
		oldCount := len(m.chat.Posts)
		oldOffset := m.chat.Viewport.YOffset()
		initial := m.chat.Page < 0
		m.chat.Posts = mm.MergePosts(m.chat.Posts, msg.posts)
		m.chat.Page = msg.page
		m.chat.HasOlder = msg.hasOlder
		for id, user := range msg.users {
			m.users[id] = user
		}
		m.ensureSelectedMessage()
		m.refreshChatContent(false)
		if initial {
			m.chat.Viewport.GotoBottom()
		} else if len(m.chat.Posts) > oldCount {
			delta := len(m.chat.Posts) - oldCount
			m.chat.Viewport.SetYOffset(oldOffset + delta*2)
		}
		return m, nil

	case postSentMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			m.errLog.Warn("send failed", "component", "mattermost", "error", msg.err)
			return m, nil
		}
		m.compose.Input.Reset()
		m.compose.CancelReply()
		if msg.post != nil && msg.post.ChannelId == m.chat.ChannelID {
			m.chat.Posts = mm.MergePosts(m.chat.Posts, []*model.Post{msg.post})
			m.refreshChatContent(false)
			m.chat.Viewport.GotoBottom()
		}
		return m, nil

	case conversationRefreshMsg:
		if msg.err != nil {
			m.errLog.Warn("conversation refresh failed", "component", "mattermost", "error", msg.err)
			return m, nil
		}
		activeID := ""
		if m.active != nil && m.active.Channel != nil {
			activeID = m.active.Channel.Id
		}
		m.conversations = msg.items
		m.active = conversationByID(m.conversations, activeID)
		m.sidebar.ApplyFilter(m.conversations)
		m.alignSidebarCursor()
		return m, nil

	case websocketMsg:
		if m.service != nil {
			cmds = append(cmds, waitEventCmd(m.service))
		}
		switch msg.event.Kind {
		case mm.EventConnected:
			m.connection = "connected"
			m.statusErr = ""
			if m.service != nil && m.user != nil && m.team != nil {
				cmds = append(cmds, refreshConversationsCmd(m.ctx, m.service, m.user, m.team))
			}
		case mm.EventReconnecting:
			m.connection = "reconnecting"
		case mm.EventOffline:
			m.connection = "offline"
			if msg.event.Err != nil {
				m.statusErr = msg.event.Err.Error()
			}
		case mm.EventRefreshConversations:
			if m.service != nil && m.user != nil && m.team != nil {
				cmds = append(cmds, refreshConversationsCmd(m.ctx, m.service, m.user, m.team))
			}
		case mm.EventPosted:
			m.handleIncomingPost(msg.event.Post)
		}
		return m, tea.Batch(cmds...)

	case markedReadMsg:
		if msg.err != nil {
			m.errLog.Warn("background operation failed", "component", "app", "channel_id", msg.channelID, "error", msg.err)
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.mode == modeSetup {
			return m.updateSetup(msg)
		}
		if m.mode != modeMain {
			return m, nil
		}
		return m.updateMainKey(msg)
	}

	if m.mode == modeSetup {
		var cmd tea.Cmd
		switch m.setup.Focus {
		case 0:
			m.setup.Server, cmd = m.setup.Server.Update(msg)
		case 1:
			m.setup.PAT, cmd = m.setup.PAT.Update(msg)
		case 2:
			m.setup.TelegramToken, cmd = m.setup.TelegramToken.Update(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else if m.mode == modeMain && m.focus == ui.FocusCompose {
		var cmd tea.Cmd
		m.compose.Input, cmd = m.compose.Input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateSetup(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.setup.Busy {
		return m, nil
	}
	if msg.String() == "ctrl+t" {
		m.setup.TelegramEnabled = !m.setup.TelegramEnabled
		if !m.setup.TelegramEnabled && m.setup.Focus == 2 {
			m.setup.Focus = 1
		}
		m.setup.Error = ""
		return m, m.focusSetupField()
	}
	switch msg.String() {
	case "tab", "shift+tab":
		fields := 2
		if m.setup.TelegramEnabled {
			fields = 3
		}
		m.setup.Server.Blur()
		m.setup.PAT.Blur()
		m.setup.TelegramToken.Blur()
		if msg.String() == "shift+tab" {
			m.setup.Focus = (m.setup.Focus + fields - 1) % fields
		} else {
			m.setup.Focus = (m.setup.Focus + 1) % fields
		}
		return m, m.focusSetupField()
	case "enter":
		url := strings.TrimRight(strings.TrimSpace(m.setup.Server.Value()), "/")
		token := strings.TrimSpace(m.setup.PAT.Value())
		telegramToken := strings.TrimSpace(m.setup.TelegramToken.Value())
		if url == "" || token == "" {
			m.setup.Error = "Server URL and Personal Access Token are required."
			return m, nil
		}
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			m.setup.Error = "Server URL must start with http:// or https://"
			return m, nil
		}
		if m.setup.TelegramEnabled && telegramToken == "" {
			m.setup.Error = "Telegram bot token is required when forwarding is enabled."
			return m, nil
		}
		m.setup.Error = ""
		m.setup.Busy = true
		return m, validateSetupCmd(m.ctx, url, token, m.setup.TelegramEnabled, telegramToken, m.cfg, m.keychain, m.infoLog, m.errLog)
	}

	var cmd tea.Cmd
	switch m.setup.Focus {
	case 0:
		m.setup.Server, cmd = m.setup.Server.Update(msg)
	case 1:
		m.setup.PAT, cmd = m.setup.PAT.Update(msg)
	case 2:
		m.setup.TelegramToken, cmd = m.setup.TelegramToken.Update(msg)
	}
	return m, cmd
}

func (m *Model) focusSetupField() tea.Cmd {
	switch m.setup.Focus {
	case 0:
		return m.setup.Server.Focus()
	case 1:
		return m.setup.PAT.Focus()
	case 2:
		return m.setup.TelegramToken.Focus()
	default:
		m.setup.Focus = 0
		return m.setup.Server.Focus()
	}
}

func (m Model) updateMainKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, m.keys.Quit) {
		m.cancel()
		if m.service != nil {
			m.service.Close()
		}
		return m, tea.Quit
	}

	if m.showHelp {
		if keyMatches(msg, m.keys.Shortcuts) || keyMatches(msg, m.keys.Cancel) || msg.String() == "enter" {
			m.showHelp = false
		}
		return m, nil
	}

	if m.sidebar.Finding {
		if keyMatches(msg, m.keys.Cancel) {
			m.sidebar.Finding = false
			m.sidebar.Query.Blur()
			m.sidebar.Query.SetValue("")
			m.sidebar.ApplyFilter(m.conversations)
			m.alignSidebarCursor()
			return m, nil
		}
		if isUpKey(msg, m.keys.Up) || msg.String() == "ctrl+p" {
			m.moveSidebar(-1)
			return m, nil
		}
		if isDownKey(msg, m.keys.Down) || msg.String() == "ctrl+n" {
			m.moveSidebar(1)
			return m, nil
		}
		switch msg.String() {
		case "escape":
			m.sidebar.Finding = false
			m.sidebar.Query.Blur()
			m.sidebar.Query.SetValue("")
			m.sidebar.ApplyFilter(m.conversations)
			m.alignSidebarCursor()
			return m, nil
		case "enter", "return":
			if selected := m.selectedConversation(); selected != nil {
				m.sidebar.Finding = false
				m.sidebar.Query.Blur()
				m.sidebar.Query.SetValue("")
				m.sidebar.ApplyFilter(m.conversations)
				m.focus = ui.FocusChat
				m.applyFocus()
				return m.openConversation(selected)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.sidebar.Query, cmd = m.sidebar.Query.Update(msg)
		m.sidebar.ApplyFilter(m.conversations)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.Shortcuts):
		m.showHelp = true
		return m, nil
	case keyMatches(msg, m.keys.Find):
		m.focus = ui.FocusSidebar
		m.applyFocus()
		m.sidebar.Finding = true
		m.sidebar.Query.SetValue("")
		m.sidebar.ApplyFilter(m.conversations)
		return m, m.sidebar.Query.Focus()
	case keyMatches(msg, m.keys.FocusNext):
		m.focus = (m.focus + 1) % 3
		return m, m.applyFocus()
	case keyMatches(msg, m.keys.FocusPrevious):
		m.focus = (m.focus + 2) % 3
		return m, m.applyFocus()
	case keyMatches(msg, m.keys.Cancel):
		if m.compose.ReplyRootID != "" {
			m.compose.CancelReply()
		}
		return m, nil
	case keyMatches(msg, m.keys.Reply):
		if m.focus == ui.FocusChat {
			m.beginReply()
			if m.compose.ReplyRootID != "" {
				m.focus = ui.FocusCompose
				return m, m.applyFocus()
			}
		}
		return m, nil
	case keyMatches(msg, m.keys.Send):
		if m.focus == ui.FocusCompose {
			return m.sendCurrentMessage()
		}
		return m, nil
	}

	switch m.focus {
	case ui.FocusSidebar:
		if isUpKey(msg, m.keys.Up) || msg.String() == "k" {
			m.moveSidebar(-1)
			return m, nil
		}
		if isDownKey(msg, m.keys.Down) || msg.String() == "j" {
			m.moveSidebar(1)
			return m, nil
		}
		switch msg.String() {
		case "enter", "return":
			if selected := m.selectedConversation(); selected != nil {
				m.focus = ui.FocusChat
				m.applyFocus()
				return m.openConversation(selected)
			}
		}
		return m, nil

	case ui.FocusChat:
		if isUpKey(msg, m.keys.Up) || msg.String() == "k" {
			m.moveMessageSelection(-1)
			m.chat.Viewport.ScrollUp(2)
			if m.chat.Viewport.AtTop() && m.chat.HasOlder && !m.chat.LoadingOlder {
				return m, m.loadOlder()
			}
			return m, nil
		}
		if isDownKey(msg, m.keys.Down) || msg.String() == "j" {
			m.moveMessageSelection(1)
			m.chat.Viewport.ScrollDown(2)
			if m.chat.Viewport.AtBottom() {
				m.chat.NewMessages = false
			}
			return m, nil
		}
		switch msg.String() {
		case "pageup":
			m.chat.Viewport.PageUp()
			if m.chat.Viewport.AtTop() && m.chat.HasOlder && !m.chat.LoadingOlder {
				return m, m.loadOlder()
			}
		case "pagedown":
			m.chat.Viewport.PageDown()
			if m.chat.Viewport.AtBottom() {
				m.chat.NewMessages = false
			}
		case "end":
			m.chat.Viewport.GotoBottom()
			m.chat.NewMessages = false
		}
		return m, nil

	case ui.FocusCompose:
		var cmd tea.Cmd
		m.compose.Input, cmd = m.compose.Input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) enterMain(session *mm.Session) (tea.Model, tea.Cmd) {
	if session == nil || session.User == nil || session.Team == nil {
		m.mode = modeSetup
		telegramToken, _ := keychain.Telegram().Get()
		m.setup = ui.NewSetup(m.cfg.ServerURL, "", m.cfg.Telegram.Enabled, telegramToken)
		m.setup.Error = "Mattermost returned an incomplete session."
		return m, nil
	}
	m.mode = modeMain
	m.user, m.team = session.User, session.Team
	m.users[m.user.Id] = m.user
	m.conversations = session.Conversations
	m.sidebar.ApplyFilter(m.conversations)
	m.connection = "reconnecting"
	m.statusErr = ""
	if m.service != nil {
		m.service.StartWebSocket(m.ctx)
	}

	chosen := conversationByID(m.conversations, m.cfg.LastConversationID)
	if chosen == nil && len(m.conversations) > 0 {
		chosen = m.conversations[0]
	}
	m.applyFocus()
	if chosen == nil {
		if m.service != nil {
			return m, waitEventCmd(m.service)
		}
		return m, nil
	}
	next, openCmd := m.openConversation(chosen)
	mmModel := next.(Model)
	var eventCmd tea.Cmd
	if mmModel.service != nil {
		eventCmd = waitEventCmd(mmModel.service)
	}
	return mmModel, tea.Batch(openCmd, eventCmd)
}

func (m Model) openConversation(conv *mm.Conversation) (tea.Model, tea.Cmd) {
	if conv == nil || conv.Channel == nil || m.service == nil || m.user == nil {
		return m, nil
	}
	m.active = conv
	m.cfg.LastConversationID = conv.Channel.Id
	conv.Unread, conv.Mention = false, false
	mm.SortConversations(m.conversations)
	m.sidebar.ApplyFilter(m.conversations)
	m.alignSidebarCursor()
	m.chat = ui.NewChat()
	m.chat.ChannelID = conv.Channel.Id
	m.chat.LoadingOlder = true
	m.compose.CancelReply()
	m.statusErr = ""
	m.resize()
	return m, tea.Batch(
		loadMessagesCmd(m.ctx, m.service, conv.Channel.Id, 0),
		markReadCmd(m.ctx, m.service, m.user.Id, conv.Channel.Id),
		saveConfigCmd(m.cfg),
	)
}

func (m Model) sendCurrentMessage() (tea.Model, tea.Cmd) {
	if m.service == nil || m.active == nil || m.active.Channel == nil {
		return m, nil
	}
	message := m.compose.Input.Value()
	if strings.TrimSpace(message) == "" {
		return m, nil
	}
	return m, sendPostCmd(m.ctx, m.service, m.active.Channel.Id, message, m.compose.ReplyRootID)
}

func (m *Model) beginReply() {
	p := m.selectedPost()
	if p == nil || isSystemPost(p) {
		return
	}
	rootID := p.Id
	if p.RootId != "" {
		rootID = p.RootId
	}
	m.compose.ReplyRootID = rootID
	author := m.displayUser(p.UserId)
	excerpt := strings.ReplaceAll(strings.TrimSpace(p.Message), "\n", " ")
	if len([]rune(excerpt)) > 48 {
		excerpt = string([]rune(excerpt)[:48]) + "…"
	}
	m.compose.ReplyLabel = fmt.Sprintf("Replying to %s: %q", author, excerpt)
}

func (m *Model) handleIncomingPost(post *model.Post) {
	if post == nil {
		return
	}
	conv := conversationByID(m.conversations, post.ChannelId)
	if conv != nil {
		if m.active == nil || m.active.Channel == nil || m.active.Channel.Id != post.ChannelId {
			conv.Unread = true
			if m.user != nil && strings.Contains(strings.ToLower(post.Message), "@"+strings.ToLower(m.user.Username)) {
				conv.Mention = true
			}
			mm.SortConversations(m.conversations)
			m.sidebar.ApplyFilter(m.conversations)
			m.alignSidebarCursor()
		}
	}
	if post.ChannelId != m.chat.ChannelID {
		return
	}
	atBottom := m.chat.Viewport.AtBottom()
	m.chat.Posts = mm.MergePosts(m.chat.Posts, []*model.Post{post})
	m.refreshChatContent(false)
	if atBottom {
		m.chat.Viewport.GotoBottom()
		m.chat.NewMessages = false
	} else {
		m.chat.NewMessages = true
	}
}

func (m *Model) moveSidebar(delta int) {
	n := len(m.sidebar.Filtered)
	if n == 0 {
		m.sidebar.Cursor = 0
		return
	}
	m.sidebar.Cursor += delta
	if m.sidebar.Cursor < 0 {
		m.sidebar.Cursor = n - 1
	} else if m.sidebar.Cursor >= n {
		m.sidebar.Cursor = 0
	}
}

func (m *Model) selectedConversation() *mm.Conversation {
	if len(m.sidebar.Filtered) == 0 || m.sidebar.Cursor < 0 || m.sidebar.Cursor >= len(m.sidebar.Filtered) {
		return nil
	}
	return m.sidebar.Filtered[m.sidebar.Cursor]
}

func (m *Model) alignSidebarCursor() {
	if m.active == nil || m.active.Channel == nil {
		return
	}
	for i, c := range m.sidebar.Filtered {
		if c != nil && c.Channel != nil && c.Channel.Id == m.active.Channel.Id {
			m.sidebar.Cursor = i
			return
		}
	}
}

func (m *Model) moveMessageSelection(delta int) {
	if len(m.chat.Posts) == 0 {
		m.chat.Selected = -1
		return
	}
	idx := m.chat.Selected
	if idx < 0 {
		if delta < 0 {
			idx = len(m.chat.Posts)
		} else {
			idx = -1
		}
	}
	for {
		idx += delta
		if idx < 0 || idx >= len(m.chat.Posts) {
			return
		}
		if !isSystemPost(m.chat.Posts[idx]) {
			m.chat.Selected = idx
			m.refreshChatContent(false)
			return
		}
	}
}

func (m *Model) ensureSelectedMessage() {
	if m.chat.Selected >= 0 && m.chat.Selected < len(m.chat.Posts) && !isSystemPost(m.chat.Posts[m.chat.Selected]) {
		return
	}
	m.chat.Selected = -1
	for i := len(m.chat.Posts) - 1; i >= 0; i-- {
		if !isSystemPost(m.chat.Posts[i]) {
			m.chat.Selected = i
			return
		}
	}
}

func (m *Model) selectedPost() *model.Post {
	if m.chat.Selected < 0 || m.chat.Selected >= len(m.chat.Posts) {
		return nil
	}
	return m.chat.Posts[m.chat.Selected]
}

func (m *Model) loadOlder() tea.Cmd {
	if m.service == nil || m.chat.ChannelID == "" || !m.chat.HasOlder || m.chat.LoadingOlder {
		return nil
	}
	m.chat.LoadingOlder = true
	return loadMessagesCmd(m.ctx, m.service, m.chat.ChannelID, m.chat.Page+1)
}

func (m *Model) applyFocus() tea.Cmd {
	m.compose.Input.Blur()
	if !m.sidebar.Finding {
		m.sidebar.Query.Blur()
	}
	if m.focus == ui.FocusCompose {
		return m.compose.Input.Focus()
	}
	return nil
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	rightWidth := max(20, m.width-33)
	contentHeight := max(4, m.height-8)
	m.chat.Viewport.SetWidth(rightWidth - 2)
	m.chat.Viewport.SetHeight(contentHeight)
	m.compose.Input.SetWidth(max(10, rightWidth-4))
}

func validateSetupCmd(ctx context.Context, url, token string, telegramEnabled bool, telegramToken string, cfg config.Config, store keychain.Store, infoLog, errLog *slog.Logger) tea.Cmd {
	return func() tea.Msg {
		svc := mm.New(url, token, infoLog, errLog)
		session, err := svc.Bootstrap(ctx)
		if err != nil {
			svc.Close()
			return setupValidatedMsg{cfg: cfg, err: err}
		}

		cfg.ServerURL = url
		cfg.Telegram.Configured = true
		cfg.Telegram.Enabled = telegramEnabled
		cfg.Telegram.ChatID = 0

		if telegramEnabled {
			chatID, pairErr := pairTelegram(ctx, telegramToken)
			if pairErr != nil {
				svc.Close()
				return setupValidatedMsg{cfg: cfg, err: pairErr}
			}
			cfg.Telegram.ChatID = chatID
		}

		if err := store.Put(token); err != nil {
			svc.Close()
			return setupValidatedMsg{cfg: cfg, err: err}
		}
		if telegramEnabled {
			if err := keychain.Telegram().Put(telegramToken); err != nil {
				svc.Close()
				return setupValidatedMsg{cfg: cfg, err: err}
			}
		}
		if err := config.Save(cfg); err != nil {
			svc.Close()
			return setupValidatedMsg{cfg: cfg, err: err}
		}
		binary, err := os.Executable()
		if err != nil {
			svc.Close()
			return setupValidatedMsg{cfg: cfg, err: fmt.Errorf("resolve KMO executable: %w", err)}
		}
		if err := launchd.InstallAndStart(binary); err != nil {
			svc.Close()
			return setupValidatedMsg{cfg: cfg, err: fmt.Errorf("restart KMO daemon: %w", err)}
		}
		return setupValidatedMsg{service: svc, session: session, cfg: cfg}
	}
}

func pairTelegram(ctx context.Context, token string) (int64, error) {
	client := tg.New(token)
	if _, err := client.GetMe(ctx); err != nil {
		return 0, fmt.Errorf("validate Telegram bot token: %w", err)
	}
	baseline, err := client.GetUpdates(ctx, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("read Telegram pairing baseline: %w", err)
	}
	var offset int64
	for _, up := range baseline {
		if up.UpdateID >= offset {
			offset = up.UpdateID + 1
		}
	}
	pairCtx, cancel := context.WithTimeout(ctx, telegramPairWindow)
	defer cancel()
	candidates := map[int64]struct{}{}
	for {
		updates, getErr := client.GetUpdates(pairCtx, offset, 10)
		if getErr != nil {
			if pairCtx.Err() != nil {
				return 0, fmt.Errorf("Telegram pairing timed out waiting for /start")
			}
			return 0, fmt.Errorf("wait for Telegram /start: %w", getErr)
		}
		for _, up := range updates {
			if up.UpdateID >= offset {
				offset = up.UpdateID + 1
			}
			if up.Message == nil {
				continue
			}
			text := strings.TrimSpace(up.Message.Text)
			if text == "/start" || strings.HasPrefix(text, "/start ") {
				candidates[up.Message.Chat.ID] = struct{}{}
			}
		}
		if len(candidates) > 1 {
			return 0, fmt.Errorf("multiple Telegram /start chats detected; pairing requires exactly one")
		}
		if len(candidates) == 1 {
			grace, graceErr := client.GetUpdates(pairCtx, offset, 2)
			if graceErr != nil && pairCtx.Err() == nil {
				return 0, fmt.Errorf("confirm Telegram pairing: %w", graceErr)
			}
			for _, up := range grace {
				if up.UpdateID >= offset {
					offset = up.UpdateID + 1
				}
				if up.Message == nil {
					continue
				}
				text := strings.TrimSpace(up.Message.Text)
				if text == "/start" || strings.HasPrefix(text, "/start ") {
					candidates[up.Message.Chat.ID] = struct{}{}
				}
			}
			if len(candidates) != 1 {
				return 0, fmt.Errorf("multiple Telegram /start chats detected; pairing requires exactly one")
			}
			var chatID int64
			for id := range candidates {
				chatID = id
			}
			if _, err := client.SendMessage(pairCtx, chatID, "KMO Telegram forwarding is actief."); err != nil {
				return 0, fmt.Errorf("send Telegram test message: %w", err)
			}
			return chatID, nil
		}
	}
}

func loadMessagesCmd(ctx context.Context, svc *mm.Service, channelID string, page int) tea.Cmd {
	return func() tea.Msg {
		posts, hasOlder, err := svc.LoadPosts(ctx, channelID, page, historyPageSize)
		if err != nil {
			return messagesLoadedMsg{channelID: channelID, page: page, err: err}
		}
		return messagesLoadedMsg{channelID: channelID, posts: posts, users: svc.ResolveUsers(ctx, posts), hasOlder: hasOlder, page: page}
	}
}

func sendPostCmd(ctx context.Context, svc *mm.Service, channelID, message, rootID string) tea.Cmd {
	return func() tea.Msg {
		post, err := svc.SendPost(ctx, channelID, message, rootID)
		return postSentMsg{post: post, err: err}
	}
}

func refreshConversationsCmd(ctx context.Context, svc *mm.Service, user *model.User, team *model.Team) tea.Cmd {
	return func() tea.Msg {
		items, err := svc.LoadConversations(ctx, user, team)
		return conversationRefreshMsg{items: items, err: err}
	}
}

func markReadCmd(ctx context.Context, svc *mm.Service, userID, channelID string) tea.Cmd {
	return func() tea.Msg {
		return markedReadMsg{channelID: channelID, err: svc.MarkRead(ctx, userID, channelID)}
	}
}

func saveConfigCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		if err := config.Save(cfg); err != nil {
			return markedReadMsg{err: fmt.Errorf("save config: %w", err)}
		}
		return nil
	}
}

func conversationByID(items []*mm.Conversation, id string) *mm.Conversation {
	if id == "" {
		return nil
	}
	for _, c := range items {
		if c != nil && c.Channel != nil && c.Channel.Id == id {
			return c
		}
	}
	return nil
}
