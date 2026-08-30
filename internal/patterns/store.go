package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ap0calypse644/git_gud/internal/coaching"
)

const historyVersion = 1

// Observation is one detector-normalized coaching candidate persisted for
// cross-match aggregation. It intentionally stores no report prose and no raw
// replay state.
type Observation struct {
	Type       string  `json:"type"`
	Confidence string  `json:"confidence"`
	StartT     float64 `json:"start_t"`
	EndT       float64 `json:"end_t"`
	GamePhase  string  `json:"game_phase"`
	Lane       string  `json:"lane,omitempty"`
}

type MatchRecord struct {
	MatchID      int64         `json:"match_id"`
	Hero         string        `json:"hero"`
	Observations []Observation `json:"observations"`
}

type PatternSummary struct {
	Type               string         `json:"type"`
	MatchesWithPattern int            `json:"matches_with_pattern"`
	Occurrences        int            `json:"occurrences"`
	MatchRate          float64        `json:"match_rate"`
	Recurring          bool           `json:"recurring"`
	Heroes             map[string]int `json:"heroes,omitempty"`
	GamePhases         map[string]int `json:"game_phases,omitempty"`
	Lanes              map[string]int `json:"lanes,omitempty"`
}

type History struct {
	Version                 int              `json:"version"`
	RecentMatchLimit        int              `json:"recent_match_limit"`
	RecentMatchesConsidered int              `json:"recent_matches_considered"`
	Matches                 []MatchRecord    `json:"matches"`
	Patterns                []PatternSummary `json:"patterns"`
}

// Store persists normalized cross-match facts to one small local JSON file.
// Record is an idempotent upsert by match ID, which makes watcher retries and
// historical reprocessing safe.
type Store struct {
	path        string
	recentLimit int
	mu          sync.Mutex
}

func NewStore(storagePath string, recentLimit int) *Store {
	return &Store{
		path:        filepath.Join(storagePath, "patterns.json"),
		recentLimit: recentLimit,
	}
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Record(input coaching.MatchCoachingInput) (string, error) {
	if s == nil {
		return "", errors.New("pattern store is nil")
	}
	if input.MatchID <= 0 {
		return "", errors.New("pattern input match_id must be positive")
	}
	if s.recentLimit <= 0 {
		return "", errors.New("pattern recent match limit must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.loadLocked()
	if err != nil {
		return "", err
	}
	record := matchRecordFromInput(input)
	upsertMatch(&history.Matches, record)
	history.RecentMatchLimit = s.recentLimit
	recomputeSummary(&history)
	if err := s.saveLocked(history); err != nil {
		return "", err
	}
	return s.path, nil
}

func (s *Store) Load() (History, error) {
	if s == nil {
		return History{}, errors.New("pattern store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	history, err := s.loadLocked()
	if err != nil {
		return History{}, err
	}
	history.RecentMatchLimit = s.recentLimit
	recomputeSummary(&history)
	return history, nil
}

func (s *Store) loadLocked() (History, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return History{
			Version:          historyVersion,
			RecentMatchLimit: s.recentLimit,
			Matches:          []MatchRecord{},
			Patterns:         []PatternSummary{},
		}, nil
	}
	if err != nil {
		return History{}, fmt.Errorf("read pattern history: %w", err)
	}
	var history History
	if err := json.Unmarshal(data, &history); err != nil {
		return History{}, fmt.Errorf("decode pattern history: %w", err)
	}
	if history.Version != historyVersion {
		return History{}, fmt.Errorf("unsupported pattern history version %d", history.Version)
	}
	if history.Matches == nil {
		history.Matches = []MatchRecord{}
	}
	if history.Patterns == nil {
		history.Patterns = []PatternSummary{}
	}
	return history, nil
}

func (s *Store) saveLocked(history History) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pattern history directory: %w", err)
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pattern history: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".patterns-*.tmp")
	if err != nil {
		return fmt.Errorf("create pattern history temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write pattern history temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync pattern history temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close pattern history temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace pattern history: %w", err)
	}
	return nil
}

func matchRecordFromInput(input coaching.MatchCoachingInput) MatchRecord {
	record := MatchRecord{
		MatchID:      input.MatchID,
		Hero:         strings.TrimSpace(input.Hero),
		Observations: make([]Observation, 0, len(input.Moments)),
	}
	for _, moment := range input.Moments {
		record.Observations = append(record.Observations, Observation{
			Type:       moment.Type,
			Confidence: moment.Confidence,
			StartT:     moment.StartT,
			EndT:       moment.EndT,
			GamePhase:  gamePhase(moment.StartT),
			Lane:       laneFromEvidence(moment.Evidence),
		})
	}
	return record
}

func upsertMatch(matches *[]MatchRecord, record MatchRecord) {
	for i := range *matches {
		if (*matches)[i].MatchID == record.MatchID {
			(*matches)[i] = record
			sort.Slice(*matches, func(i, j int) bool { return (*matches)[i].MatchID < (*matches)[j].MatchID })
			return
		}
	}
	*matches = append(*matches, record)
	sort.Slice(*matches, func(i, j int) bool { return (*matches)[i].MatchID < (*matches)[j].MatchID })
}

func recomputeSummary(history *History) {
	if history == nil {
		return
	}
	limit := history.RecentMatchLimit
	if limit <= 0 {
		limit = 1
	}
	start := len(history.Matches) - limit
	if start < 0 {
		start = 0
	}
	recent := history.Matches[start:]
	history.RecentMatchesConsidered = len(recent)

	type accumulator struct {
		matches    int
		occurrences int
		heroes     map[string]int
		phases     map[string]int
		lanes      map[string]int
	}
	acc := make(map[string]*accumulator)
	for _, match := range recent {
		seenInMatch := make(map[string]bool)
		for _, observation := range match.Observations {
			if strings.TrimSpace(observation.Type) == "" {
				continue
			}
			a := acc[observation.Type]
			if a == nil {
				a = &accumulator{
					heroes: make(map[string]int),
					phases: make(map[string]int),
					lanes:  make(map[string]int),
				}
				acc[observation.Type] = a
			}
			a.occurrences++
			if match.Hero != "" {
				a.heroes[match.Hero]++
			}
			if observation.GamePhase != "" {
				a.phases[observation.GamePhase]++
			}
			if observation.Lane != "" {
				a.lanes[observation.Lane]++
			}
			if !seenInMatch[observation.Type] {
				a.matches++
				seenInMatch[observation.Type] = true
			}
		}
	}

	patterns := make([]PatternSummary, 0, len(acc))
	for typ, a := range acc {
		rate := 0.0
		if len(recent) > 0 {
			rate = float64(a.matches) / float64(len(recent))
		}
		patterns = append(patterns, PatternSummary{
			Type:               typ,
			MatchesWithPattern: a.matches,
			Occurrences:        a.occurrences,
			MatchRate:          rate,
			Recurring:          a.matches >= 2,
			Heroes:             a.heroes,
			GamePhases:         a.phases,
			Lanes:              a.lanes,
		})
	}
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].MatchesWithPattern != patterns[j].MatchesWithPattern {
			return patterns[i].MatchesWithPattern > patterns[j].MatchesWithPattern
		}
		if patterns[i].Occurrences != patterns[j].Occurrences {
			return patterns[i].Occurrences > patterns[j].Occurrences
		}
		return patterns[i].Type < patterns[j].Type
	})
	history.Patterns = patterns
}

// The phase buckets are intentionally coarse and stable. They are not claims
// about a specific patch's strategic meta; they are grouping keys for habit
// recurrence only.
func gamePhase(t float64) string {
	switch {
	case t < 0:
		return "pregame"
	case t < 10*60:
		return "early"
	case t < 25*60:
		return "mid"
	default:
		return "late"
	}
}

func laneFromEvidence(evidence any) string {
	switch value := evidence.(type) {
	case coaching.PostWaveOverstayReviewEvidence:
		return value.Lane
	case *coaching.PostWaveOverstayReviewEvidence:
		if value != nil {
			return value.Lane
		}
	}
	return ""
}
