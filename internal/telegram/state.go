package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const mappingTTL = 30 * 24 * time.Hour

type Mapping struct {
	ChannelID string    `json:"channel_id"`
	CreatedAt time.Time `json:"created_at"`
}

type State struct {
	mu           sync.Mutex
	path         string
	Mappings     map[string]Mapping `json:"mappings"`
	LastUpdateID int64              `json:"last_update_id,omitempty"`
}

func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "kmo", "telegram.json"), nil
}

func LoadState() (*State, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}
	s := &State{path: path, Mappings: map[string]Mapping{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("read Telegram state: %w", err)
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, s); err != nil {
			return nil, fmt.Errorf("decode Telegram state: %w", err)
		}
	}
	if s.Mappings == nil {
		s.Mappings = map[string]Mapping{}
	}
	_ = s.Cleanup(time.Now())
	return s, nil
}

func (s *State) Put(messageID int64, channelID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Mappings[strconv.FormatInt(messageID, 10)] = Mapping{ChannelID: channelID, CreatedAt: now.UTC()}
	s.cleanupLocked(now)
	return s.saveLocked()
}

func (s *State) Resolve(messageID int64, now time.Time) (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strconv.FormatInt(messageID, 10)
	m, ok := s.Mappings[key]
	if !ok {
		return "", false, false
	}
	if now.Sub(m.CreatedAt) > mappingTTL {
		delete(s.Mappings, key)
		_ = s.saveLocked()
		return "", false, true
	}
	return m.ChannelID, true, false
}

func (s *State) Offset() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastUpdateID <= 0 {
		return 0
	}
	return s.LastUpdateID + 1
}

func (s *State) AdvanceUpdate(updateID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if updateID <= s.LastUpdateID {
		return nil
	}
	s.LastUpdateID = updateID
	return s.saveLocked()
}

func (s *State) Cleanup(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.cleanupLocked(now)
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func (s *State) cleanupLocked(now time.Time) bool {
	changed := false
	for key, m := range s.Mappings {
		if now.Sub(m.CreatedAt) > mappingTTL {
			delete(s.Mappings, key)
			changed = true
		}
	}
	return changed
}

func (s *State) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
