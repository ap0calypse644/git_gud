package storage

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

type MatchStatus string

const (
	StatusDiscovered        MatchStatus = "discovered"
	StatusMetadataReady     MatchStatus = "metadata_ready"
	StatusReplayWaiting     MatchStatus = "replay_waiting"
	StatusReplayDownloaded  MatchStatus = "replay_downloaded"
	StatusTimelineReady     MatchStatus = "timeline_ready"
	StatusCoachingReady     MatchStatus = "coaching_ready"
	StatusReplayUnavailable MatchStatus = "replay_unavailable"
)

type MatchState struct {
	MatchID          int64       `json:"match_id"`
	StartTime        int64       `json:"start_time,omitempty"`
	Status           MatchStatus `json:"status"`
	ParseRequestedAt *time.Time  `json:"parse_requested_at,omitempty"`
	LastAttemptAt    *time.Time  `json:"last_attempt_at,omitempty"`
	ReplayPath       string      `json:"replay_path,omitempty"`
	TimelinePath     string      `json:"timeline_path,omitempty"`
	ReportPath       string      `json:"report_path,omitempty"`
	PatternRecorded  bool        `json:"pattern_recorded,omitempty"`
	LastError        string      `json:"last_error,omitempty"`
}

type State struct {
	Initialized     bool                   `json:"initialized"`
	LastSeenMatchID int64                  `json:"last_seen_match_id"`
	Matches         map[string]*MatchState `json:"matches"`
}

func NewState() State {
	return State{Matches: make(map[string]*MatchState)}
}

func (s *State) Ensure() {
	if s.Matches == nil {
		s.Matches = make(map[string]*MatchState)
	}
}

func (s *State) Match(matchID int64) *MatchState {
	s.Ensure()
	return s.Matches[strconv.FormatInt(matchID, 10)]
}

func (s *State) Put(m *MatchState) {
	s.Ensure()
	s.Matches[strconv.FormatInt(m.MatchID, 10)] = m
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store { return &Store{path: path} }

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(state)
}

func (s *Store) loadLocked() (State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return NewState(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}

	state := NewState()
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	state.Ensure()
	return state, nil
}

func (s *Store) saveLocked(state State) error {
	state.Ensure()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create state temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write state temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync state temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close state temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}
