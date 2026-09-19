package telegram

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateResolveAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	s := &State{path: filepath.Join(t.TempDir(), "telegram.json"), Mappings: map[string]Mapping{}}
	if err := s.Put(42, "channel-a", now); err != nil {
		t.Fatal(err)
	}
	channel, ok, expired := s.Resolve(42, now.Add(24*time.Hour))
	if !ok || expired || channel != "channel-a" {
		t.Fatalf("unexpected mapping: channel=%q ok=%v expired=%v", channel, ok, expired)
	}
	_, ok, expired = s.Resolve(42, now.Add(31*24*time.Hour))
	if ok || !expired {
		t.Fatalf("expected expired mapping")
	}
}
