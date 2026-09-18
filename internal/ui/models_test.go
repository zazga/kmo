package ui

import (
	"testing"

	mm "github.com/zazga/kmo/internal/mattermost"
)

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		candidate string
		query     string
		want      bool
	}{
		{"development", "dev", true},
		{"general-chat", "gch", true},
		{"alice smith alice", "asm", true},
		{"Kevin", "Kevn", true},
		{"Kevin de Vries kevin", "kevn", true},
		{"random", "zzz", false},
	}
	for _, tc := range cases {
		if got := FuzzyMatch(tc.candidate, tc.query); got != tc.want {
			t.Fatalf("FuzzyMatch(%q,%q)=%v want %v", tc.candidate, tc.query, got, tc.want)
		}
	}
}

func TestSidebarWidth(t *testing.T) {
	if got := NewSidebar().Width; got != 50 {
		t.Fatalf("sidebar width=%d want 50", got)
	}
}

func TestFinderFilterClearsWhenFinderCloses(t *testing.T) {
	s := NewSidebar()
	all := []*mm.Conversation{
		{Display: "Kevin", SearchName: "kevin"},
		{Display: "Alice", SearchName: "alice"},
	}
	s.Finding = true
	s.Query.SetValue("Kevn")
	s.ApplyFilter(all)
	if len(s.Filtered) != 1 || s.Filtered[0].Display != "Kevin" {
		t.Fatalf("finder result=%v want Kevin", s.Filtered)
	}

	s.Finding = false
	s.ApplyFilter(all)
	if got := len(s.Filtered); got != 2 {
		t.Fatalf("filtered count after closing finder=%d want 2", got)
	}
	if got := s.Query.Value(); got != "" {
		t.Fatalf("query after closing finder=%q want empty", got)
	}
}
