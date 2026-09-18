package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/zazga/kmo/internal/config"
	mm "github.com/zazga/kmo/internal/mattermost"
	"github.com/zazga/kmo/internal/ui"
)

func TestMessageSelectionSkipsSystemPosts(t *testing.T) {
	m := Model{chat: ui.NewChat()}
	m.chat.Posts = []*model.Post{
		{Id: "u1", UserId: "a", Message: "one"},
		{Id: "s1", Type: "system_join_channel", Message: "joined"},
		{Id: "u2", UserId: "b", Message: "two"},
	}
	m.chat.Selected = 0

	m.moveMessageSelection(1)
	if m.chat.Selected != 2 {
		t.Fatalf("selected=%d, want 2", m.chat.Selected)
	}
	m.moveMessageSelection(-1)
	if m.chat.Selected != 0 {
		t.Fatalf("selected=%d, want 0", m.chat.Selected)
	}
}

func TestBeginReplyUsesThreadRoot(t *testing.T) {
	m := Model{chat: ui.NewChat(), compose: ui.NewCompose(), users: map[string]*model.User{}}
	m.users["u"] = &model.User{Id: "u", Username: "alice"}
	m.chat.Posts = []*model.Post{{Id: "reply", RootId: "root", UserId: "u", Message: "hello"}}
	m.chat.Selected = 0

	m.beginReply()
	if m.compose.ReplyRootID != "root" {
		t.Fatalf("root=%q, want root", m.compose.ReplyRootID)
	}
	if m.compose.ReplyLabel == "" {
		t.Fatal("reply label should be populated")
	}
}

func TestTruncateName(t *testing.T) {
	got := truncateName("abcdefghijklmnopqrstuvwxyz", 12)
	if got != "abcdefg....." {
		t.Fatalf("got %q", got)
	}
}

func TestSidebarPhysicalArrowNavigation(t *testing.T) {
	m := Model{mode: modeMain, focus: ui.FocusSidebar, sidebar: ui.NewSidebar()}
	m.keys = newKeymap(config.Defaults().Keybindings)
	m.sidebar.Filtered = []*mm.Conversation{
		{Channel: &model.Channel{Id: "a"}, Display: "A"},
		{Channel: &model.Channel{Id: "b"}, Display: "B"},
	}
	next, _ := m.updateMainKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got := next.(Model)
	if got.sidebar.Cursor != 1 {
		t.Fatalf("down arrow cursor=%d, want 1", got.sidebar.Cursor)
	}
	next, _ = got.updateMainKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got = next.(Model)
	if got.sidebar.Cursor != 0 {
		t.Fatalf("up arrow cursor=%d, want 0", got.sidebar.Cursor)
	}
}

func TestSearchPhysicalArrowNavigation(t *testing.T) {
	m := Model{mode: modeMain, focus: ui.FocusSidebar, sidebar: ui.NewSidebar()}
	m.keys = newKeymap(config.Defaults().Keybindings)
	m.sidebar.Finding = true
	m.sidebar.Filtered = []*mm.Conversation{
		{Channel: &model.Channel{Id: "a"}, Display: "A"},
		{Channel: &model.Channel{Id: "b"}, Display: "B"},
	}
	next, _ := m.updateMainKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got := next.(Model)
	if got.sidebar.Cursor != 1 {
		t.Fatalf("search down arrow cursor=%d, want 1", got.sidebar.Cursor)
	}
}
