package app

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
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
