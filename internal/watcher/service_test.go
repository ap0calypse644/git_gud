package watcher

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/processor"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

type fakeAPI struct {
	recent []opendota.RecentMatch
}

func (f *fakeAPI) RecentMatches(context.Context, uint32) ([]opendota.RecentMatch, error) {
	return f.recent, nil
}

type fakeProcessor struct {
	calls []int64
}

func (f *fakeProcessor) Process(_ context.Context, matchID int64, _ bool) (processor.Result, error) {
	f.calls = append(f.calls, matchID)
	return processor.Result{Status: storage.StatusReplayDownloaded}, nil
}

func testConfig() config.Config {
	cfg := config.Config{}
	cfg.Player.AccountID = 256161923
	cfg.Poll.Interval = config.Duration(time.Minute)
	cfg.Replays.RetryInterval = config.Duration(time.Second)
	cfg.Replays.RetryFor = config.Duration(168 * time.Hour)
	return cfg
}

func TestFirstRunEstablishesBaseline(t *testing.T) {
	cfg := testConfig()
	api := &fakeAPI{recent: []opendota.RecentMatch{{MatchID: 40}, {MatchID: 42}}}
	matchProcessor := &fakeProcessor{}
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	svc := New(cfg, api, matchProcessor, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastSeenMatchID != 42 || len(state.Matches) != 0 {
		t.Fatalf("state = %#v", state)
	}
	if len(matchProcessor.calls) != 0 {
		t.Fatalf("processor calls = %#v", matchProcessor.calls)
	}
}

func TestNewMatchDelegatesToProcessor(t *testing.T) {
	cfg := testConfig()
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	initial := storage.NewState()
	initial.Initialized = true
	initial.LastSeenMatchID = 42
	if err := store.Save(initial); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{recent: []opendota.RecentMatch{{MatchID: 43, StartTime: time.Now().Unix()}}}
	matchProcessor := &fakeProcessor{}
	svc := New(cfg, api, matchProcessor, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(matchProcessor.calls) != 1 || matchProcessor.calls[0] != 43 {
		t.Fatalf("processor calls = %#v", matchProcessor.calls)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastSeenMatchID != 43 {
		t.Fatalf("last seen = %d", state.LastSeenMatchID)
	}
	if got := state.Match(43); got == nil || got.Status != storage.StatusDiscovered {
		t.Fatalf("discovered match state = %#v", got)
	}
}

func TestTimelineReadyIsPendingWhenCoachingEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.Coaching.Enabled = true
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	state := storage.NewState()
	state.Initialized = true
	state.Put(&storage.MatchState{MatchID: 50, Status: storage.StatusTimelineReady})
	state.Put(&storage.MatchState{MatchID: 51, Status: storage.StatusCoachingReady})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	matchProcessor := &fakeProcessor{}
	svc := New(cfg, &fakeAPI{}, matchProcessor, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(matchProcessor.calls) != 1 || matchProcessor.calls[0] != 50 {
		t.Fatalf("processor calls = %#v", matchProcessor.calls)
	}
}

func TestTimelineReadyIsTerminalWhenCoachingDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.Coaching.Enabled = false
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	state := storage.NewState()
	state.Initialized = true
	state.Put(&storage.MatchState{MatchID: 52, Status: storage.StatusTimelineReady})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	matchProcessor := &fakeProcessor{}
	svc := New(cfg, &fakeAPI{}, matchProcessor, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(matchProcessor.calls) != 0 {
		t.Fatalf("processor calls = %#v", matchProcessor.calls)
	}
}
