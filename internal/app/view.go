package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattermost/mattermost/server/public/model"
	mm "github.com/zazga/kmo/internal/mattermost"
	"github.com/zazga/kmo/internal/ui"
)

var (
	accent      = lipgloss.Color("#154273")
	borderColor = lipgloss.Color("#666666")
)

func (m Model) View() tea.View {
	var content string
	switch m.mode {
	case modeSetup:
		content = m.viewSetup()
	case modeConnecting:
		content = m.centered("Connecting to Mattermost…")
	case modeMain:
		content = m.viewMain()
	default:
		content = m.centered("Starting KMO…")
	}
	if m.showHelp {
		content = m.viewHelp(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "KMO — Mattermost"
	v.KeyboardEnhancements.ReportAlternateKeys = true
	v.KeyboardEnhancements.ReportAllKeysAsEscapeCodes = true
	v.KeyboardEnhancements.ReportAssociatedText = true
	return v
}

func (m Model) viewSetup() string {
	title := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("KMO setup")
	subtitle := "Connect one Mattermost account. Your PAT is stored in macOS Keychain."
	body := []string{title, subtitle, "", m.setup.Server.View(), m.setup.PAT.View(), ""}
	if m.setup.Busy {
		body = append(body, lipgloss.NewStyle().Foreground(accent).Render("Validating Mattermost credentials…"))
	} else {
		body = append(body, "Tab switch field  •  Enter connect")
	}
	if m.setup.Error != "" {
		body = append(body, "", lipgloss.NewStyle().Bold(true).Render(m.setup.Error))
	}
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(1, 2).Render(strings.Join(body, "\n"))
	return m.centered(panel)
}

func (m Model) viewMain() string {
	if m.width > 0 && (m.width < 72 || m.height < 16) {
		return m.centered("KMO needs at least 72×16 terminal cells.")
	}
	left := m.viewSidebar()
	right := m.viewChatPane()
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	status := m.viewStatus()
	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

func (m Model) viewSidebar() string {
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)
	if m.sidebar.Finding {
		b.WriteString(m.sidebar.Query.View())
		b.WriteString("\n")
	} else {
		b.WriteString(titleStyle.Render("Conversations"))
		b.WriteString("\n")
	}

	currentKind := mm.ConversationKind(-1)
	for i, c := range m.sidebar.Filtered {
		if c == nil || c.Channel == nil {
			continue
		}
		if c.Kind != currentKind {
			currentKind = c.Kind
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(titleStyle.Render(sectionName(c.Kind)))
			b.WriteString("\n")
		}
		marker := "  "
		if c.Mention {
			marker = "@ "
		} else if c.Unread {
			marker = "• "
		}
		name := truncateName(c.Display, 25)
		line := marker + name
		if i == m.sidebar.Cursor {
			line = lipgloss.NewStyle().Foreground(accent).Bold(true).Render("› " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(m.sidebar.Filtered) == 0 {
		b.WriteString("  No matches\n")
	}

	style := lipgloss.NewStyle().Width(30).Height(max(1, m.height-3)).BorderRight(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(borderColor).Padding(0, 1)
	if m.focus == ui.FocusSidebar {
		style = style.BorderForeground(accent)
	}
	return style.Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) viewChatPane() string {
	width := max(20, m.width-33)
	header := "No conversation"
	if m.active != nil {
		header = conversationHeader(m.active)
	}
	headerStyle := lipgloss.NewStyle().Foreground(accent).Bold(true).Width(width-2).Padding(0, 1)
	chatStyle := lipgloss.NewStyle().Width(width-2).Height(max(4, m.height-8)).Padding(0, 1)
	if m.focus == ui.FocusChat {
		chatStyle = chatStyle.BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(accent)
	}

	composeParts := []string{}
	if m.compose.ReplyLabel != "" {
		composeParts = append(composeParts, lipgloss.NewStyle().Foreground(accent).Render(m.compose.ReplyLabel))
	}
	composeParts = append(composeParts, m.compose.Input.View())
	composeStyle := lipgloss.NewStyle().Width(width-2).BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(borderColor).Padding(0, 1)
	if m.focus == ui.FocusCompose {
		composeStyle = composeStyle.BorderForeground(accent)
	}

	parts := []string{
		headerStyle.Render(header),
		chatStyle.Render(m.chat.Viewport.View()),
	}
	if m.chat.NewMessages {
		parts = append(parts, lipgloss.NewStyle().Foreground(accent).Bold(true).PaddingLeft(1).Render("↓ new messages"))
	}
	parts = append(parts, composeStyle.Render(strings.Join(composeParts, "\n")))
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m Model) viewStatus() string {
	state := m.connection
	if state == "" {
		state = "offline"
	}
	left := "Mattermost: " + state
	if m.statusErr != "" && state != "connected" {
		left += " — " + oneLine(m.statusErr, 60)
	}
	hint := m.keys.Shortcuts + " shortcuts"
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(hint)-2)
	return lipgloss.NewStyle().Width(max(1, m.width)).Foreground(accent).Render(left + strings.Repeat(" ", gap) + hint)
}

func (m Model) viewHelp(background string) string {
	lines := []string{
		"KMO shortcuts",
		"",
		fmt.Sprintf("%-16s %s", m.keys.Find, "find conversation"),
		fmt.Sprintf("%-16s %s", m.keys.Reply, "reply in thread"),
		fmt.Sprintf("%-16s %s", m.keys.Send, "send message"),
		fmt.Sprintf("%-16s %s", m.keys.FocusNext, "next pane"),
		fmt.Sprintf("%-16s %s", m.keys.FocusPrevious, "previous pane"),
		"j / k           navigate",
		"arrows          navigate",
		"Esc             cancel current mode",
		"",
		"Press shortcut, Esc or Enter to close",
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(1, 2).Render(strings.Join(lines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Center, box, "", background)
}

func (m Model) centered(s string) string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, s)
}

func (m *Model) refreshChatContent(_ bool) {
	if m.chat.ChannelID == "" {
		m.chat.Viewport.SetContent("")
		return
	}
	m.chat.Viewport.SetContent(m.renderChatContent())
}

func (m Model) renderChatContent() string {
	if len(m.chat.Posts) == 0 {
		if m.chat.LoadingOlder {
			return "Loading messages…"
		}
		return "No messages yet."
	}
	replyCounts := map[string]int{}
	for _, p := range m.chat.Posts {
		if p != nil && p.RootId != "" {
			replyCounts[p.RootId]++
		}
	}

	var b strings.Builder
	for i, p := range m.chat.Posts {
		if p == nil {
			continue
		}
		if isSystemPost(p) {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render("· " + oneLine(p.Message, max(20, m.chat.Viewport.Width()-4))))
			b.WriteString("\n")
			continue
		}
		selected := i == m.chat.Selected && m.focus == ui.FocusChat
		prefix := ""
		if p.RootId != "" {
			prefix = "   ↳ "
		} else if replyCounts[p.Id] > 0 {
			prefix = "▸ "
		}
		header := fmt.Sprintf("%s%s %s", prefix, m.displayUser(p.UserId), formatTime(p.CreateAt))
		headerStyle := lipgloss.NewStyle().Bold(true)
		if m.user != nil && p.UserId == m.user.Id {
			headerStyle = headerStyle.Foreground(accent)
		}
		if selected {
			headerStyle = headerStyle.Underline(true).Foreground(accent)
		}
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		textPrefix := ""
		if p.RootId != "" {
			textPrefix = "     "
		} else {
			textPrefix = "  "
		}
		message := strings.ReplaceAll(p.Message, "\r\n", "\n")
		for _, line := range strings.Split(message, "\n") {
			b.WriteString(textPrefix + line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) displayUser(id string) string {
	if u := m.users[id]; u != nil {
		if u.FirstName != "" || u.LastName != "" {
			if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
				return name
			}
		}
		if u.Nickname != "" {
			return u.Nickname
		}
		if u.Username != "" {
			return u.Username
		}
	}
	if id == "" {
		return "system"
	}
	return "unknown"
}

func conversationHeader(c *mm.Conversation) string {
	if c == nil {
		return "No conversation"
	}
	switch c.Kind {
	case mm.ConversationDM:
		return "@ " + c.Display
	case mm.ConversationGroup:
		return "group " + c.Display
	default:
		return "# " + c.Display
	}
}

func sectionName(kind mm.ConversationKind) string {
	switch kind {
	case mm.ConversationDM:
		return "DMs"
	case mm.ConversationGroup:
		return "Groups"
	default:
		return "Channels"
	}
}

func truncateName(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	rs := []rune(s)
	keep := max(1, maxRunes-5)
	return string(rs[:keep]) + "....."
}

func formatTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms).Local()
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func isSystemPost(p *model.Post) bool {
	return p != nil && strings.HasPrefix(p.Type, "system_")
}

func oneLine(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	rs := []rune(s)
	return string(rs[:max(1, maxRunes-1)]) + "…"
}
