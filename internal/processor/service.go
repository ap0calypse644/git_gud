package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

// API is the subset of OpenDota used after a match ID is known.
type API interface {
	Match(context.Context, int64) (opendota.Match, error)
	RequestParse(context.Context, int64) error
}

type Acquirer interface {
	Acquire(context.Context, opendota.Match) (string, error)
}

type TimelineBuilder interface {
	Build(context.Context, opendota.Match, string) (string, error)
}

// Coach is deliberately compact-only: raw MatchTimeline values are converted
// to MatchCoachingInput before this boundary is crossed.
type Coach interface {
	Generate(context.Context, coaching.MatchCoachingInput) (string, error)
}

// PatternRecorder receives the same detector-normalized compact input. It must
// not derive recurrence from report prose or raw replay state.
type PatternRecorder interface {
	Record(coaching.MatchCoachingInput) (string, error)
}

type StateStore interface {
	Load() (storage.State, error)
	Save(storage.State) error
}

// Result describes how far a single invocation managed to advance a match.
// A nil error does not necessarily mean processing is complete; callers should
// inspect Status and the artifact paths.
type Result struct {
	Match        opendota.Match
	Status       storage.MatchStatus
	ReplayPath   string
	TimelinePath string
	ReportPath   string
}

// Service owns processing after a match ID is known. Automatic discovery and
// explicit historical processing both call this same service so their behavior
// cannot drift apart.
type Service struct {
	cfg      config.Config
	api      API
	acquirer Acquirer
	timeline TimelineBuilder
	coach    Coach
	patterns PatternRecorder
	store    StateStore
	log      *slog.Logger
	now      func() time.Time
}

// New preserves the deterministic/replay-only construction seam used by tests
// and diagnostic callers.
func New(cfg config.Config, api API, acquirer Acquirer, timelineBuilder TimelineBuilder, store StateStore, logger *slog.Logger) *Service {
	return NewWithCoachAndPatterns(cfg, api, acquirer, timelineBuilder, nil, nil, store, logger)
}

// NewWithCoach preserves the Phase G construction seam while allowing Phase H
// callers to opt into pattern recording separately.
func NewWithCoach(cfg config.Config, api API, acquirer Acquirer, timelineBuilder TimelineBuilder, coach Coach, store StateStore, logger *slog.Logger) *Service {
	return NewWithCoachAndPatterns(cfg, api, acquirer, timelineBuilder, coach, nil, store, logger)
}

func NewWithCoachAndPatterns(cfg config.Config, api API, acquirer Acquirer, timelineBuilder TimelineBuilder, coach Coach, patterns PatternRecorder, store StateStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:      cfg,
		api:      api,
		acquirer: acquirer,
		timeline: timelineBuilder,
		coach:    coach,
		patterns: patterns,
		store:    store,
		log:      logger,
		now:      time.Now,
	}
}

// Process advances one match through metadata, replay acquisition, replay
// timeline generation, normalized pattern persistence, and (when configured)
// coaching. force bypasses retry throttling and is used by explicit -match
// invocations. It never modifies State.LastSeenMatchID, so historical
// processing cannot disturb automatic match discovery.
func (s *Service) Process(ctx context.Context, matchID int64, force bool) (Result, error) {
	state, err := s.store.Load()
	if err != nil {
		return Result{}, err
	}

	m := state.Match(matchID)
	if m == nil {
		m = &storage.MatchState{MatchID: matchID, Status: storage.StatusDiscovered}
		state.Put(m)
		if err := s.store.Save(state); err != nil {
			return Result{}, err
		}
	}

	// Phase H can be enabled after a match already reached coaching_ready. Backfill
	// normalized pattern facts from the durable timeline without re-running AI or
	// any replay acquisition work.
	if m.Status == storage.StatusCoachingReady {
		if s.patterns != nil && !m.PatternRecorded && m.TimelinePath != "" {
			input, err := s.loadCoachingInput(m)
			if err != nil {
				m.LastError = err.Error()
				_ = s.store.Save(state)
				return Result{}, err
			}
			if err := s.recordPatterns(state, m, input); err != nil {
				return Result{}, err
			}
		}
		return resultForState(opendota.Match{}, m), nil
	}

	now := s.now().UTC()
	if !force && m.LastAttemptAt != nil && now.Sub(*m.LastAttemptAt) < s.cfg.Replays.RetryInterval.Duration() {
		return resultForState(opendota.Match{}, m), nil
	}

	m.LastAttemptAt = &now
	m.LastError = ""

	// A timeline is a durable deterministic artifact. If a previous coaching or
	// pattern attempt failed, resume directly from it instead of re-fetching
	// metadata, re-downloading the replay, or rebuilding the timeline.
	if m.Status == storage.StatusTimelineReady && m.TimelinePath != "" {
		return s.continueFromTimeline(ctx, state, m, opendota.Match{})
	}

	match, err := s.api.Match(ctx, matchID)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(state)
		return Result{}, fmt.Errorf("fetch metadata: %w", err)
	}
	m.StartTime = match.StartTime
	m.Status = storage.StatusMetadataReady

	if replay.ResolveURL(match) == "" {
		if s.retryExpired(match.StartTime, now) {
			m.Status = storage.StatusReplayUnavailable
			m.LastError = "replay retry window expired before replay metadata became available"
			if err := s.store.Save(state); err != nil {
				return Result{}, err
			}
			s.log.Warn("replay unavailable", "match_id", matchID, "reason", m.LastError)
			return resultForState(match, m), nil
		}

		m.Status = storage.StatusReplayWaiting
		if s.cfg.Replays.RequestParse && m.ParseRequestedAt == nil {
			if err := s.api.RequestParse(ctx, matchID); err != nil {
				m.LastError = err.Error()
				_ = s.store.Save(state)
				return Result{}, fmt.Errorf("request OpenDota parse: %w", err)
			}
			requestedAt := now
			m.ParseRequestedAt = &requestedAt
			s.log.Info("requested OpenDota parse", "match_id", matchID)
		}
		if err := s.store.Save(state); err != nil {
			return Result{}, err
		}
		return resultForState(match, m), nil
	}

	path, err := s.acquirer.Acquire(ctx, match)
	if err != nil {
		m.Status = storage.StatusReplayWaiting
		m.LastError = err.Error()
		if s.retryExpired(match.StartTime, now) && (replay.IsStatus(err, http.StatusNotFound) || replay.IsStatus(err, http.StatusForbidden)) {
			m.Status = storage.StatusReplayUnavailable
		}
		_ = s.store.Save(state)
		return Result{}, fmt.Errorf("acquire replay: %w", err)
	}

	m.Status = storage.StatusReplayDownloaded
	m.ReplayPath = path
	m.LastError = ""
	if err := s.store.Save(state); err != nil {
		return Result{}, err
	}
	s.log.Info("replay downloaded", "match_id", matchID, "path", path)

	// Keeping this interface optional makes the acquisition layer testable in
	// isolation and leaves a clean seam for degraded non-replay analysis later.
	if s.timeline == nil {
		return resultForState(match, m), nil
	}

	timelinePath, err := s.timeline.Build(ctx, match, path)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(state)
		return Result{}, fmt.Errorf("build replay timeline: %w", err)
	}
	m.Status = storage.StatusTimelineReady
	m.TimelinePath = timelinePath
	m.PatternRecorded = false
	m.LastError = ""
	if err := s.store.Save(state); err != nil {
		return Result{}, err
	}
	s.log.Info("replay timeline ready", "match_id", matchID, "path", timelinePath)

	return s.continueFromTimeline(ctx, state, m, match)
}

func (s *Service) continueFromTimeline(ctx context.Context, state storage.State, m *storage.MatchState, match opendota.Match) (Result, error) {
	input, err := s.loadCoachingInput(m)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(state)
		return Result{}, err
	}

	if s.patterns != nil && !m.PatternRecorded {
		if err := s.recordPatterns(state, m, input); err != nil {
			return Result{}, err
		}
	}

	if s.coach == nil {
		if err := s.store.Save(state); err != nil {
			return Result{}, err
		}
		return resultForState(match, m), nil
	}
	return s.generateCoaching(ctx, state, m, match, input)
}

func (s *Service) loadCoachingInput(m *storage.MatchState) (coaching.MatchCoachingInput, error) {
	f, err := os.Open(m.TimelinePath)
	if err != nil {
		return coaching.MatchCoachingInput{}, fmt.Errorf("open replay timeline for analysis: %w", err)
	}
	defer f.Close()

	var tl timeline.MatchTimeline
	if err := json.NewDecoder(f).Decode(&tl); err != nil {
		return coaching.MatchCoachingInput{}, fmt.Errorf("decode replay timeline for analysis: %w", err)
	}
	input := coaching.BuildMatchCoachingInput(&tl)
	if input.MatchID != m.MatchID {
		return coaching.MatchCoachingInput{}, fmt.Errorf("coaching input match_id %d does not match state match_id %d", input.MatchID, m.MatchID)
	}
	return input, nil
}

func (s *Service) recordPatterns(state storage.State, m *storage.MatchState, input coaching.MatchCoachingInput) error {
	path, err := s.patterns.Record(input)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(state)
		return fmt.Errorf("record recurring patterns: %w", err)
	}
	m.PatternRecorded = true
	m.LastError = ""
	if err := s.store.Save(state); err != nil {
		return err
	}
	s.log.Info("pattern history updated", "match_id", m.MatchID, "path", path)
	return nil
}

func (s *Service) generateCoaching(ctx context.Context, state storage.State, m *storage.MatchState, match opendota.Match, input coaching.MatchCoachingInput) (Result, error) {
	reportPath, err := s.coach.Generate(ctx, input)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(state)
		return Result{}, fmt.Errorf("generate coaching report: %w", err)
	}
	m.Status = storage.StatusCoachingReady
	m.ReportPath = reportPath
	m.LastError = ""
	if err := s.store.Save(state); err != nil {
		return Result{}, err
	}
	s.log.Info("coaching report ready", "match_id", m.MatchID, "path", reportPath)
	return resultForState(match, m), nil
}

func resultForState(match opendota.Match, state *storage.MatchState) Result {
	return Result{
		Match:        match,
		Status:       state.Status,
		ReplayPath:   state.ReplayPath,
		TimelinePath: state.TimelinePath,
		ReportPath:   state.ReportPath,
	}
}

func (s *Service) retryExpired(startTime int64, now time.Time) bool {
	if startTime <= 0 {
		return false
	}
	return now.Sub(time.Unix(startTime, 0)) > s.cfg.Replays.RetryFor.Duration()
}
