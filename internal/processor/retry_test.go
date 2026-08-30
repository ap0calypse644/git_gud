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

func TestProcessAutomaticRetriesBackOffAndResetOnProgress(t *testing.T) {
	cfg := testConfig()
	cfg.Replays.RetryInterval = config.Duration(time.Minute)
	cfg.Replays.RetryMaxInterval = config.Duration(4 * time.Minute)

	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	current := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	api := &fakeAPI{matches: map[int64]opendota.Match{
		60: {MatchID: 60, StartTime: current.Unix()},
	}}
	acquirer := &fakeAcquirer{}
	svc := New(cfg, api, acquirer, nil, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.now = func() time.Time { return current }

	result, err := svc.Process(context.Background(), 60, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusReplayWaiting {
		t.Fatalf("status=%s", result.Status)
	}
	assertRetryState(t, store, 60, 1, current.Add(time.Minute))
	if len(api.matchCalls) != 1 || len(api.parseRequests) != 1 {
		t.Fatalf("calls match=%v parse=%v", api.matchCalls, api.parseRequests)
	}

	// Before the scheduled retry, automatic processing is a no-op.
	if _, err := svc.Process(context.Background(), 60, false); err != nil {
		t.Fatal(err)
	}
	if len(api.matchCalls) != 1 {
		t.Fatalf("automatic retry ignored backoff: calls=%v", api.matchCalls)
	}

	current = current.Add(time.Minute)
	if _, err := svc.Process(context.Background(), 60, false); err != nil {
		t.Fatal(err)
	}
	assertRetryState(t, store, 60, 2, current.Add(2*time.Minute))

	current = current.Add(2 * time.Minute)
	if _, err := svc.Process(context.Background(), 60, false); err != nil {
		t.Fatal(err)
	}
	assertRetryState(t, store, 60, 3, current.Add(4*time.Minute))

	current = current.Add(4 * time.Minute)
	if _, err := svc.Process(context.Background(), 60, false); err != nil {
		t.Fatal(err)
	}
	assertRetryState(t, store, 60, 4, current.Add(4*time.Minute))

	// Replay metadata becoming available is progress: the waiting backoff is
	// cleared before acquisition, and a successful acquisition leaves no retry.
	current = current.Add(4 * time.Minute)
	api.matches[60] = opendota.Match{MatchID: 60, StartTime: current.Add(-11 * time.Minute).Unix(), ReplayURL: "https://example.test/60.dem.bz2"}
	result, err = svc.Process(context.Background(), 60, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusReplayDownloaded {
		t.Fatalf("status=%s", result.Status)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	match := state.Match(60)
	if match == nil || match.RetryCount != 0 || match.NextRetryAt != nil {
		t.Fatalf("retry state after progress=%#v", match)
	}
	if len(api.parseRequests) != 1 {
		t.Fatalf("parse request repeated: %v", api.parseRequests)
	}
}

func TestCoachingReadyPatternFailureObeysAutomaticBackoff(t *testing.T) {
	cfg := testConfig()
	cfg.Replays.RetryInterval = config.Duration(time.Minute)
	cfg.Replays.RetryMaxInterval = config.Duration(4 * time.Minute)

	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 61)
	state := storage.NewState()
	state.Put(&storage.MatchState{MatchID: 61, Status: storage.StatusCoachingReady, TimelinePath: timelinePath})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	patterns := &fakePatternRecorder{err: errPatternDisk}
	svc := NewWithCoachAndPatterns(cfg, &fakeAPI{matches: map[int64]opendota.Match{}}, &fakeAcquirer{}, &fakeTimelineBuilder{}, nil, patterns, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	current := time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return current }

	if _, err := svc.Process(context.Background(), 61, false); err == nil {
		t.Fatal("pattern failure unexpectedly succeeded")
	}
	assertRetryState(t, store, 61, 1, current.Add(time.Minute))
	if len(patterns.calls) != 1 {
		t.Fatalf("pattern calls=%d", len(patterns.calls))
	}

	// This used to retry every watcher poll because coaching_ready backfill ran
	// before the retry throttle. It now shares the same persisted schedule.
	if _, err := svc.Process(context.Background(), 61, false); err != nil {
		t.Fatal(err)
	}
	if len(patterns.calls) != 1 {
		t.Fatalf("pattern backfill ignored backoff: calls=%d", len(patterns.calls))
	}
}

var errPatternDisk = patternDiskError("pattern disk failure")

type patternDiskError string

func (e patternDiskError) Error() string { return string(e) }

func assertRetryState(t *testing.T, store *storage.Store, matchID int64, wantCount int, wantNext time.Time) {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	match := state.Match(matchID)
	if match == nil {
		t.Fatalf("match %d missing", matchID)
	}
	if match.RetryCount != wantCount {
		t.Fatalf("retry count=%d want=%d", match.RetryCount, wantCount)
	}
	if match.NextRetryAt == nil || !match.NextRetryAt.Equal(wantNext) {
		t.Fatalf("next retry=%v want=%s", match.NextRetryAt, wantNext)
	}
}
