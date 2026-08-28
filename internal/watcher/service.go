package watcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

type API interface {
	RecentMatches(context.Context, uint32) ([]opendota.RecentMatch, error)
	Match(context.Context, int64) (opendota.Match, error)
	RequestParse(context.Context, int64) error
}

type Acquirer interface {
	Acquire(context.Context, opendota.Match) (string, error)
}

type StateStore interface {
	Load() (storage.State, error)
	Save(storage.State) error
}

type Service struct {
	cfg      config.Config
	api      API
	acquirer Acquirer
	store    StateStore
	log      *slog.Logger
	now      func() time.Time
}

func New(cfg config.Config, api API, acquirer Acquirer, store StateStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{cfg: cfg, api: api, acquirer: acquirer, store: store, log: logger, now: time.Now}
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.RunOnce(ctx); err != nil {
		s.log.Error("watch cycle failed", "error", err)
	}

	ticker := time.NewTicker(s.cfg.Poll.Interval.Duration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				s.log.Error("watch cycle failed", "error", err)
			}
		}
	}
}

func (s *Service) RunOnce(ctx context.Context) error {
	state, err := s.store.Load()
	if err != nil {
		return err
	}

	recent, err := s.api.RecentMatches(ctx, s.cfg.Player.AccountID)
	if err != nil {
		return fmt.Errorf("fetch recent matches: %w", err)
	}
	sort.Slice(recent, func(i, j int) bool { return recent[i].MatchID < recent[j].MatchID })

	if !state.Initialized {
		state.Initialized = true
		if !s.cfg.Watcher.BootstrapExisting {
			for _, m := range recent {
				if m.MatchID > state.LastSeenMatchID {
					state.LastSeenMatchID = m.MatchID
				}
			}
			if err := s.store.Save(state); err != nil {
				return err
			}
			s.log.Info("match baseline established", "last_seen_match_id", state.LastSeenMatchID)
			return s.processPending(ctx, &state)
		}
	}

	for _, m := range recent {
		if m.MatchID <= state.LastSeenMatchID {
			continue
		}
		state.Put(&storage.MatchState{MatchID: m.MatchID, StartTime: m.StartTime, Status: storage.StatusDiscovered})
		state.LastSeenMatchID = m.MatchID
		s.log.Info("new match discovered", "match_id", m.MatchID, "hero_id", m.HeroID)
	}
	if err := s.store.Save(state); err != nil {
		return err
	}
	return s.processPending(ctx, &state)
}

func (s *Service) ProcessMatch(ctx context.Context, matchID int64) error {
	state, err := s.store.Load()
	if err != nil {
		return err
	}
	m := state.Match(matchID)
	if m == nil {
		m = &storage.MatchState{MatchID: matchID, Status: storage.StatusDiscovered}
		state.Put(m)
		if err := s.store.Save(state); err != nil {
			return err
		}
	}
	return s.processOne(ctx, &state, m, true)
}

func (s *Service) processPending(ctx context.Context, state *storage.State) error {
	ids := make([]int64, 0, len(state.Matches))
	for _, m := range state.Matches {
		if m.Status == storage.StatusReplayDownloaded || m.Status == storage.StatusReplayUnavailable {
			continue
		}
		ids = append(ids, m.MatchID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var errs []error
	for _, id := range ids {
		m := state.Match(id)
		if err := s.processOne(ctx, state, m, false); err != nil {
			errs = append(errs, fmt.Errorf("match %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) processOne(ctx context.Context, state *storage.State, m *storage.MatchState, force bool) error {
	now := s.now().UTC()
	if !force && m.LastAttemptAt != nil && now.Sub(*m.LastAttemptAt) < s.cfg.Replays.RetryInterval.Duration() {
		return nil
	}
	m.LastAttemptAt = &now
	m.LastError = ""

	match, err := s.api.Match(ctx, m.MatchID)
	if err != nil {
		m.LastError = err.Error()
		_ = s.store.Save(*state)
		return fmt.Errorf("fetch metadata: %w", err)
	}
	m.StartTime = match.StartTime
	m.Status = storage.StatusMetadataReady

	if replay.ResolveURL(match) == "" {
		if s.retryExpired(match.StartTime, now) {
			m.Status = storage.StatusReplayUnavailable
			m.LastError = "replay retry window expired before replay metadata became available"
			if err := s.store.Save(*state); err != nil {
				return err
			}
			s.log.Warn("replay unavailable", "match_id", m.MatchID, "reason", m.LastError)
			return nil
		}
		m.Status = storage.StatusReplayWaiting
		if s.cfg.Replays.RequestParse && m.ParseRequestedAt == nil {
			if err := s.api.RequestParse(ctx, m.MatchID); err != nil {
				m.LastError = err.Error()
				_ = s.store.Save(*state)
				return fmt.Errorf("request OpenDota parse: %w", err)
			}
			requestedAt := now
			m.ParseRequestedAt = &requestedAt
			s.log.Info("requested OpenDota parse", "match_id", m.MatchID)
		}
		return s.store.Save(*state)
	}

	path, err := s.acquirer.Acquire(ctx, match)
	if err != nil {
		m.Status = storage.StatusReplayWaiting
		m.LastError = err.Error()
		if s.retryExpired(match.StartTime, now) && (replay.IsStatus(err, http.StatusNotFound) || replay.IsStatus(err, http.StatusForbidden)) {
			m.Status = storage.StatusReplayUnavailable
		}
		_ = s.store.Save(*state)
		return fmt.Errorf("acquire replay: %w", err)
	}
	m.Status = storage.StatusReplayDownloaded
	m.ReplayPath = path
	m.LastError = ""
	if err := s.store.Save(*state); err != nil {
		return err
	}
	s.log.Info("replay downloaded", "match_id", m.MatchID, "path", path)
	return nil
}

func (s *Service) retryExpired(startTime int64, now time.Time) bool {
	if startTime <= 0 {
		return false
	}
	return now.Sub(time.Unix(startTime, 0)) > s.cfg.Replays.RetryFor.Duration()
}
