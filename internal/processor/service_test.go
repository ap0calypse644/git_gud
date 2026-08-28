package processor

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

type fakeAPI struct {
	matches       map[int64]opendota.Match
	parseRequests []int64
}

func (f *fakeAPI) Match(_ context.Context, id int64) (opendota.Match, error) {
	return f.matches[id], nil
}

func (f *fakeAPI) RequestParse(_ context.Context, id int64) error {
	f.parseRequests = append(f.parseRequests, id)
	return nil
}

type fakeAcquirer struct {
	calls []int64
}

func (f *fakeAcquirer) Acquire(_ context.Context, match opendota.Match) (string, error) {
	f.calls = append(f.calls, match.MatchID)
	return filepath.Join("data", "replays", "test.dem"), nil
}

type fakeTimelineBuilder struct {
	calls []int64
}

func (f *fakeTimelineBuilder) Build(_ context.Context, match opendota.Match, _ string) (string, error) {
	f.calls = append(f.calls, match.MatchID)
	return filepath.Join("data", "timelines", "test.json"), nil
}

func testConfig() config.Config {
	cfg := config.Config{}
	cfg.Player.AccountID = 256161923
	cfg.Replays.RequestParse = true
	cfg.Replays.RetryInterval = config.Duration(time.Minute)
	cfg.Replays.RetryFor = config.Duration(168 * time.Hour)
	return cfg
}

func TestProcessDoesNotChangeWatcherBaseline(t *testing.T) {
	cfg := testConfig()
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 100
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{matches: map[int64]opendota.Match{
		42: {MatchID: 42, StartTime: time.Now().Unix(), ReplayURL: "https://example.test/42.dem.bz2"},
	}}
	acquirer := &fakeAcquirer{}
	timelineBuilder := &fakeTimelineBuilder{}
	svc := New(cfg, api, acquirer, timelineBuilder, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 42, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusTimelineReady || result.ReplayPath == "" || result.TimelinePath == "" {
		t.Fatalf("result = %#v", result)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSeenMatchID != 100 {
		t.Fatalf("manual processing changed baseline: got %d want 100", got.LastSeenMatchID)
	}
	if got.Match(42) == nil || got.Match(42).Status != storage.StatusTimelineReady {
		t.Fatalf("match state = %#v", got.Match(42))
	}
	if len(timelineBuilder.calls) != 1 || timelineBuilder.calls[0] != 42 {
		t.Fatalf("timeline calls = %#v", timelineBuilder.calls)
	}
}

func TestProcessRequestsParseThenAcquiresReplay(t *testing.T) {
	cfg := testConfig()
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()
	api := &fakeAPI{matches: map[int64]opendota.Match{
		43: {MatchID: 43, StartTime: now.Unix()},
	}}
	acquirer := &fakeAcquirer{}
	svc := New(cfg, api, acquirer, nil, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 43, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusReplayWaiting {
		t.Fatalf("status = %q", result.Status)
	}
	if len(api.parseRequests) != 1 || api.parseRequests[0] != 43 {
		t.Fatalf("parse requests = %#v", api.parseRequests)
	}

	api.matches[43] = opendota.Match{MatchID: 43, StartTime: now.Unix(), ReplayURL: "https://example.test/43.dem.bz2"}
	result, err = svc.Process(context.Background(), 43, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusReplayDownloaded {
		t.Fatalf("status = %q", result.Status)
	}
	if len(acquirer.calls) != 1 || acquirer.calls[0] != 43 {
		t.Fatalf("acquirer calls = %#v", acquirer.calls)
	}
}
