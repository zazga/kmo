package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessageShort(t *testing.T) {
	parts := SplitMessage("Kevin · 09:42", "hello\nworld")
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0] != "Kevin · 09:42\nhello\nworld" {
		t.Fatalf("unexpected message: %q", parts[0])
	}
}

func TestSplitMessageLong(t *testing.T) {
	parts := SplitMessage("Kevin · 09:42", strings.Repeat("x", 9000))
	if len(parts) < 2 {
		t.Fatalf("expected split message")
	}
	for i, part := range parts {
		if !strings.Contains(part, "(") {
			t.Fatalf("part %d missing index", i)
		}
	}
}
