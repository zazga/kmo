package mattermost

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestSortConversations(t *testing.T) {
	items := []*Conversation{
		{Kind: ConversationChannel, Display: "Zulu"},
		{Kind: ConversationChannel, Display: "Alpha", Unread: true},
		{Kind: ConversationDM, Display: "Bob", Unread: true},
		{Kind: ConversationDM, Display: "Alice"},
		{Kind: ConversationGroup, Display: "Project"},
	}

	SortConversations(items)

	want := []string{"Alpha", "Zulu", "Bob", "Alice", "Project"}
	for i, name := range want {
		if items[i].Display != name {
			t.Fatalf("position %d: got %q want %q", i, items[i].Display, name)
		}
	}
}

func TestMergePostsDeduplicatesAndSorts(t *testing.T) {
	a := &model.Post{Id: "a", CreateAt: 20, Message: "old"}
	b := &model.Post{Id: "b", CreateAt: 10}
	aNew := &model.Post{Id: "a", CreateAt: 20, Message: "new"}

	got := MergePosts([]*model.Post{a}, []*model.Post{b, aNew})
	if len(got) != 2 {
		t.Fatalf("got %d posts, want 2", len(got))
	}
	if got[0].Id != "b" || got[1].Id != "a" {
		t.Fatalf("unexpected order: %s, %s", got[0].Id, got[1].Id)
	}
	if got[1].Message != "new" {
		t.Fatalf("duplicate should keep incoming post; got %q", got[1].Message)
	}
}
