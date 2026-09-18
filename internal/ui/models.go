package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"github.com/mattermost/mattermost/server/public/model"
	mm "github.com/zazga/kmo/internal/mattermost"
)

type Focus int

const (
	FocusSidebar Focus = iota
	FocusChat
	FocusCompose
)

type Sidebar struct {
	Cursor int
	Query textinput.Model
	Finding bool
	Width int
	Filtered []*mm.Conversation
}

func NewSidebar() Sidebar {
	q := textinput.New()
	q.Placeholder = "find..."
	q.Prompt = "/ "
	q.CharLimit = 120
	return Sidebar{Query: q, Width: 30}
}

type Chat struct {
	Viewport viewport.Model
	Posts []*model.Post
	Selected int
	Page int
	HasOlder bool
	LoadingOlder bool
	NewMessages bool
	ChannelID string
}

func NewChat() Chat {
	return Chat{Viewport: viewport.New(viewport.WithWidth(1), viewport.WithHeight(1)), Selected: -1, Page: -1, HasOlder: true}
}

type Compose struct { Input textarea.Model; ReplyRootID, ReplyLabel string }

func NewCompose() Compose {
	input := textarea.New()
	input.Placeholder = "Write a message…"
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.CharLimit = 8000
	return Compose{Input: input}
}

func (c *Compose) CancelReply() { c.ReplyRootID = ""; c.ReplyLabel = "" }

type Setup struct { Server, PAT textinput.Model; Focus int; Error string; Busy bool }

func NewSetup(initialURL, initialPAT string) Setup {
	server := textinput.New(); server.Placeholder = "https://mattermost.example.com"; server.Prompt = "Server  "; server.SetValue(initialURL); server.Focus(); server.SetWidth(60)
	pat := textinput.New(); pat.Placeholder = "Personal Access Token"; pat.Prompt = "PAT     "; pat.SetValue(initialPAT); pat.EchoMode = textinput.EchoPassword; pat.EchoCharacter = '•'; pat.SetWidth(60)
	return Setup{Server: server, PAT: pat}
}

func (s *Sidebar) ApplyFilter(all []*mm.Conversation) {
	query := strings.TrimSpace(strings.ToLower(s.Query.Value()))
	if query == "" { s.Filtered = append(s.Filtered[:0], all...) } else {
		s.Filtered = s.Filtered[:0]
		for _, c := range all { if FuzzyMatch(strings.ToLower(c.Display+" "+c.SearchName), query) { s.Filtered = append(s.Filtered, c) } }
	}
	if len(s.Filtered) == 0 { s.Cursor = 0 } else if s.Cursor >= len(s.Filtered) { s.Cursor = len(s.Filtered)-1 }
}

func ApplyTheme(sidebar *Sidebar, compose *Compose, setup *Setup, isDark bool) {
	if sidebar != nil { sidebar.Query.SetStyles(textinput.DefaultStyles(isDark)) }
	if compose != nil { compose.Input.SetStyles(textarea.DefaultStyles(isDark)) }
	if setup != nil { setup.Server.SetStyles(textinput.DefaultStyles(isDark)); setup.PAT.SetStyles(textinput.DefaultStyles(isDark)) }
}

func FuzzyMatch(candidate, query string) bool {
	candidate = strings.ToLower(candidate); query = strings.ToLower(query)
	if strings.Contains(candidate, query) { return true }
	qr := []rune(query); qi := 0
	for _, r := range candidate { if qi < len(qr) && r == qr[qi] { qi++; if qi == len(qr) { return true } } }
	return len(qr) == 0
}
