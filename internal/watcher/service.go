package watcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/processor"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

// API is deliberately discovery-only. Once a match ID is known, all work is
// delegated to processor.Service so manual and automatic processing share the
// same implementation.
type API interface {
	RecentMatches(context.Context, uint32) ([]opendota.RecentMatch, error)
}

type MatchProcessor interface {
	Process(context.Context, int64, bool) (processor.Result, error)
}

type StateStore interface {
	Load() (storage.State, error)
	Save(storage.State) error
}

type Service struct {
	cfg       config.Config
	api       API
	processor MatchProcessor
	store     StateStore
	log       *slog.Logger
}

func New(cfg config.Config, api API, matchProcessor MatchProcessor, store StateStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{cfg: cfg, api: api, processor: matchProcessor, store: store, log: logger}
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
			return s.processPending(ctx)
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
	return s.processPending(ctx)
}

func (s *Service) processPending(ctx context.Context) error {
	state, err := s.store.Load()
	if err != nil {
		return err
	}

	ids := make([]int64, 0, len(state.Matches))
	for _, m := range state.Matches {
		if m.Status == storage.StatusTimelineReady || m.Status == storage.StatusReplayUnavailable {
			continue
		}
		ids = append(ids, m.MatchID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var errs []error
	for _, id := range ids {
		if _, err := s.processor.Process(ctx, id, false); err != nil {
			errs = append(errs, fmt.Errorf("match %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}
