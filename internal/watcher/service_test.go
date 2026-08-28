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
	"github.com/ap0calypse644/git_gud/internal/storage"
)

type fakeAPI struct {
	recent        []opendota.RecentMatch
	matches       map[int64]opendota.Match
	parseRequests []int64
}

func (f *fakeAPI) RecentMatches(context.Context, uint32) ([]opendota.RecentMatch, error) {
	return f.recent, nil
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

func testConfig() config.Config {
	cfg := config.Config{}
	cfg.Player.AccountID = 256161923
	cfg.Poll.Interval = config.Duration(time.Minute)
	cfg.Replays.RequestParse = true
	cfg.Replays.RetryInterval = config.Duration(time.Second)
	cfg.Replays.RetryFor = config.Duration(168 * time.Hour)
	return cfg
}

func TestFirstRunEstablishesBaseline(t *testing.T) {
	cfg := testConfig()
	api := &fakeAPI{recent: []opendota.RecentMatch{{MatchID: 40}, {MatchID: 42}}, matches: map[int64]opendota.Match{}}
	acquirer := &fakeAcquirer{}
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	svc := New(cfg, api, acquirer, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
}

func TestNewMatchRequestsParseThenDownloads(t *testing.T) {
	cfg := testConfig()
	store := storage.New(filepath.Join(t.TempDir(), "state.json"))
	initial := storage.NewState()
	initial.Initialized = true
	initial.LastSeenMatchID = 42
	if err := store.Save(initial); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{
		recent:  []opendota.RecentMatch{{MatchID: 43, StartTime: time.Now().Unix()}},
		matches: map[int64]opendota.Match{43: {MatchID: 43, StartTime: time.Now().Unix()}},
	}
	acquirer := &fakeAcquirer{}
	svc := New(cfg, api, acquirer, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load()
	if got := state.Match(43); got == nil || got.Status != storage.StatusReplayWaiting || got.ParseRequestedAt == nil {
		t.Fatalf("after parse request: %#v", got)
	}
	if len(api.parseRequests) != 1 || api.parseRequests[0] != 43 {
		t.Fatalf("parse requests = %#v", api.parseRequests)
	}

	api.matches[43] = opendota.Match{MatchID: 43, StartTime: time.Now().Unix(), ReplayURL: "https://example.test/43.dem.bz2"}
	if err := svc.ProcessMatch(context.Background(), 43); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load()
	if got := state.Match(43); got == nil || got.Status != storage.StatusReplayDownloaded {
		t.Fatalf("after download: %#v", got)
	}
	if len(acquirer.calls) != 1 || acquirer.calls[0] != 43 {
		t.Fatalf("acquirer calls = %#v", acquirer.calls)
	}
}
