package processor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
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
}

// Service owns processing after a match ID is known. Automatic discovery and
// explicit historical processing both call this same service so their behavior
// cannot drift apart.
type Service struct {
	cfg      config.Config
	api      API
	acquirer Acquirer
	timeline TimelineBuilder
	store    StateStore
	log      *slog.Logger
	now      func() time.Time
}

func New(cfg config.Config, api API, acquirer Acquirer, timelineBuilder TimelineBuilder, store StateStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:      cfg,
		api:      api,
		acquirer: acquirer,
		timeline: timelineBuilder,
		store:    store,
		log:      logger,
		now:      time.Now,
	}
}

// Process advances one match through metadata, replay acquisition, and replay
// timeline generation. force bypasses retry throttling and is used by explicit
// -match invocations. It never modifies State.LastSeenMatchID, so historical
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

	now := s.now().UTC()
	if !force && m.LastAttemptAt != nil && now.Sub(*m.LastAttemptAt) < s.cfg.Replays.RetryInterval.Duration() {
		return resultForState(opendota.Match{}, m), nil
	}

	m.LastAttemptAt = &now
	m.LastError = ""

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
	m.LastError = ""
	if err := s.store.Save(state); err != nil {
		return Result{}, err
	}
	s.log.Info("replay timeline ready", "match_id", matchID, "path", timelinePath)
	return resultForState(match, m), nil
}

func resultForState(match opendota.Match, state *storage.MatchState) Result {
	return Result{
		Match:        match,
		Status:       state.Status,
		ReplayPath:   state.ReplayPath,
		TimelinePath: state.TimelinePath,
	}
}

func (s *Service) retryExpired(startTime int64, now time.Time) bool {
	if startTime <= 0 {
		return false
	}
	return now.Sub(time.Unix(startTime, 0)) > s.cfg.Replays.RetryFor.Duration()
}
