package ui

import (
	"strings"
	"unicode"

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
	Cursor   int
	Query    textinput.Model
	Finding  bool
	Width    int
	Filtered []*mm.Conversation
}

func NewSidebar() Sidebar {
	q := textinput.New()
	q.Placeholder = "find..."
	q.Prompt = "/ "
	q.CharLimit = 120
	return Sidebar{Query: q, Width: 50}
}

type Chat struct {
	Viewport     viewport.Model
	Posts        []*model.Post
	Selected     int
	Page         int
	HasOlder     bool
	LoadingOlder bool
	NewMessages  bool
	ChannelID    string
}

func NewChat() Chat {
	return Chat{Viewport: viewport.New(viewport.WithWidth(1), viewport.WithHeight(1)), Selected: -1, Page: -1, HasOlder: true}
}

type Compose struct {
	Input                   textarea.Model
	ReplyRootID, ReplyLabel string
}

func NewCompose() Compose {
	input := textarea.New()
	input.Placeholder = "Write a message…"
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.CharLimit = 8000
	return Compose{Input: input}
}

func (c *Compose) CancelReply() { c.ReplyRootID = ""; c.ReplyLabel = "" }

type Setup struct {
	Server, PAT, TelegramToken textinput.Model
	TelegramEnabled            bool
	Focus                      int
	Error                      string
	Busy                       bool
}

func NewSetup(initialURL, initialPAT string, telegramArgs ...any) Setup {
	telegramEnabled := false
	telegramToken := ""
	if len(telegramArgs) > 0 {
		if v, ok := telegramArgs[0].(bool); ok {
			telegramEnabled = v
		}
	}
	if len(telegramArgs) > 1 {
		if v, ok := telegramArgs[1].(string); ok {
			telegramToken = v
		}
	}
	server := textinput.New()
	server.Placeholder = "https://mattermost.example.com"
	server.Prompt = "Server    "
	server.SetValue(initialURL)
	server.Focus()
	server.SetWidth(60)
	pat := textinput.New()
	pat.Placeholder = "Personal Access Token"
	pat.Prompt = "PAT       "
	pat.SetValue(initialPAT)
	pat.EchoMode = textinput.EchoPassword
	pat.EchoCharacter = '•'
	pat.SetWidth(60)
	tg := textinput.New()
	tg.Placeholder = "Telegram bot token"
	tg.Prompt = "Telegram  "
	tg.SetValue(telegramToken)
	tg.EchoMode = textinput.EchoPassword
	tg.EchoCharacter = '•'
	tg.SetWidth(60)
	return Setup{Server: server, PAT: pat, TelegramToken: tg, TelegramEnabled: telegramEnabled}
}

func (s *Sidebar) ApplyFilter(all []*mm.Conversation) {
	query := ""
	if s.Finding {
		query = strings.TrimSpace(strings.ToLower(s.Query.Value()))
	} else if s.Query.Value() != "" {
		s.Query.SetValue("")
	}
	if query == "" {
		s.Filtered = append(s.Filtered[:0], all...)
	} else {
		s.Filtered = s.Filtered[:0]
		for _, c := range all {
			if c != nil && FuzzyMatch(strings.ToLower(c.Display+" "+c.SearchName), query) {
				s.Filtered = append(s.Filtered, c)
			}
		}
	}
	if len(s.Filtered) == 0 {
		s.Cursor = 0
	} else if s.Cursor >= len(s.Filtered) {
		s.Cursor = len(s.Filtered) - 1
	}
}

func ApplyTheme(sidebar *Sidebar, compose *Compose, setup *Setup, isDark bool) {
	if sidebar != nil {
		sidebar.Query.SetStyles(textinput.DefaultStyles(isDark))
	}
	if compose != nil {
		compose.Input.SetStyles(textarea.DefaultStyles(isDark))
	}
	if setup != nil {
		setup.Server.SetStyles(textinput.DefaultStyles(isDark))
		setup.PAT.SetStyles(textinput.DefaultStyles(isDark))
		setup.TelegramToken.SetStyles(textinput.DefaultStyles(isDark))
	}
}

func FuzzyMatch(candidate, query string) bool {
	candidate = strings.TrimSpace(strings.ToLower(candidate))
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}
	if strings.Contains(candidate, query) || isSubsequence(candidate, query) {
		return true
	}
	maxDistance := 1
	if len([]rune(query)) >= 7 {
		maxDistance = 2
	}
	for _, token := range strings.FieldsFunc(candidate, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token != "" && levenshteinWithin(token, query, maxDistance) {
			return true
		}
	}
	return false
}

func isSubsequence(candidate, query string) bool {
	qr := []rune(query)
	qi := 0
	for _, r := range candidate {
		if qi < len(qr) && r == qr[qi] {
			qi++
			if qi == len(qr) {
				return true
			}
		}
	}
	return false
}

func levenshteinWithin(a, b string, maxDistance int) bool {
	ar, br := []rune(a), []rune(b)
	if abs(len(ar)-len(br)) > maxDistance {
		return false
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr := make([]int, len(br)+1)
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			curr[j] = minInt(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if curr[j] < rowMin {
				rowMin = curr[j]
			}
		}
		if rowMin > maxDistance {
			return false
		}
		prev = curr
	}
	return prev[len(br)] <= maxDistance
}

func minInt(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
