package processor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

type fakeAPI struct {
	matches       map[int64]opendota.Match
	matchCalls    []int64
	parseRequests []int64
}

func (f *fakeAPI) Match(_ context.Context, id int64) (opendota.Match, error) {
	f.matchCalls = append(f.matchCalls, id)
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

type fakeCoach struct {
	calls []coaching.MatchCoachingInput
	path  string
	err   error
}

func (f *fakeCoach) Generate(_ context.Context, input coaching.MatchCoachingInput) (string, error) {
	f.calls = append(f.calls, input)
	return f.path, f.err
}

type fakePatternRecorder struct {
	calls []coaching.MatchCoachingInput
	path  string
	err   error
}

func (f *fakePatternRecorder) Record(input coaching.MatchCoachingInput) (string, error) {
	f.calls = append(f.calls, input)
	return f.path, f.err
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

func TestProcessResumesTimelineReadyDirectlyIntoCoaching(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 44)
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 100
	state.Put(&storage.MatchState{MatchID: 44, Status: storage.StatusTimelineReady, TimelinePath: timelinePath})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{matches: map[int64]opendota.Match{}}
	acquirer := &fakeAcquirer{}
	timelineBuilder := &fakeTimelineBuilder{}
	coach := &fakeCoach{path: filepath.Join(root, "reports", "44.json")}
	svc := NewWithCoach(cfg, api, acquirer, timelineBuilder, coach, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 44, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusCoachingReady || result.ReportPath != coach.path {
		t.Fatalf("result=%#v", result)
	}
	if len(coach.calls) != 1 || coach.calls[0].MatchID != 44 || coach.calls[0].Hero != "slark" {
		t.Fatalf("coach calls=%#v", coach.calls)
	}
	if len(api.matchCalls) != 0 || len(acquirer.calls) != 0 || len(timelineBuilder.calls) != 0 {
		t.Fatalf("upstream work repeated: api=%v acquire=%v timeline=%v", api.matchCalls, acquirer.calls, timelineBuilder.calls)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSeenMatchID != 100 {
		t.Fatalf("coaching changed watcher baseline: got %d want 100", got.LastSeenMatchID)
	}
	if match := got.Match(44); match == nil || match.Status != storage.StatusCoachingReady || match.ReportPath != coach.path {
		t.Fatalf("match state=%#v", match)
	}
}

func TestProcessCoachingFailureKeepsTimelineReady(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 45)
	state := storage.NewState()
	state.Put(&storage.MatchState{MatchID: 45, Status: storage.StatusTimelineReady, TimelinePath: timelinePath})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	coach := &fakeCoach{err: errors.New("temporary provider failure")}
	svc := NewWithCoach(cfg, &fakeAPI{matches: map[int64]opendota.Match{}}, &fakeAcquirer{}, &fakeTimelineBuilder{}, coach, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.Process(context.Background(), 45, true); err == nil {
		t.Fatal("coaching failure unexpectedly succeeded")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	match := got.Match(45)
	if match == nil || match.Status != storage.StatusTimelineReady {
		t.Fatalf("match state=%#v", match)
	}
	if match.ReportPath != "" || match.LastError != "temporary provider failure" {
		t.Fatalf("failed coaching state=%#v", match)
	}
}

func TestProcessRecordsPatternsBeforeCoaching(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 46)
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 100
	state.Put(&storage.MatchState{MatchID: 46, Status: storage.StatusTimelineReady, TimelinePath: timelinePath})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	patterns := &fakePatternRecorder{path: filepath.Join(root, "patterns.json")}
	coach := &fakeCoach{path: filepath.Join(root, "reports", "46.json")}
	svc := NewWithCoachAndPatterns(cfg, &fakeAPI{matches: map[int64]opendota.Match{}}, &fakeAcquirer{}, &fakeTimelineBuilder{}, coach, patterns, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 46, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusCoachingReady {
		t.Fatalf("result=%#v", result)
	}
	if len(patterns.calls) != 1 || patterns.calls[0].MatchID != 46 {
		t.Fatalf("pattern calls=%#v", patterns.calls)
	}
	if len(coach.calls) != 1 || coach.calls[0].MatchID != 46 {
		t.Fatalf("coach calls=%#v", coach.calls)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	match := got.Match(46)
	if match == nil || !match.PatternRecorded || match.Status != storage.StatusCoachingReady {
		t.Fatalf("match state=%#v", match)
	}
	if got.LastSeenMatchID != 100 {
		t.Fatalf("pattern recording changed watcher baseline: got %d want 100", got.LastSeenMatchID)
	}
}

func TestProcessBackfillsPatternsForCoachingReadyMatchWithoutAI(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 47)
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 100
	state.Put(&storage.MatchState{
		MatchID:      47,
		Status:       storage.StatusCoachingReady,
		TimelinePath: timelinePath,
		ReportPath:   filepath.Join(root, "reports", "47.json"),
	})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{matches: map[int64]opendota.Match{}}
	acquirer := &fakeAcquirer{}
	timelineBuilder := &fakeTimelineBuilder{}
	patterns := &fakePatternRecorder{path: filepath.Join(root, "patterns.json")}
	svc := NewWithCoachAndPatterns(cfg, api, acquirer, timelineBuilder, nil, patterns, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 47, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusCoachingReady || len(patterns.calls) != 1 || patterns.calls[0].MatchID != 47 {
		t.Fatalf("result=%#v patterns=%#v", result, patterns.calls)
	}
	if len(api.matchCalls) != 0 || len(acquirer.calls) != 0 || len(timelineBuilder.calls) != 0 {
		t.Fatalf("backfill repeated upstream work: api=%v acquire=%v timeline=%v", api.matchCalls, acquirer.calls, timelineBuilder.calls)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if match := got.Match(47); match == nil || !match.PatternRecorded || match.Status != storage.StatusCoachingReady {
		t.Fatalf("match state=%#v", match)
	}
	if got.LastSeenMatchID != 100 {
		t.Fatalf("backfill changed watcher baseline: got %d want 100", got.LastSeenMatchID)
	}
}

func TestProcessPatternFailureLeavesMatchRetryable(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 48)
	state := storage.NewState()
	state.Put(&storage.MatchState{MatchID: 48, Status: storage.StatusCoachingReady, TimelinePath: timelinePath})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	patterns := &fakePatternRecorder{err: errors.New("pattern disk failure")}
	svc := NewWithCoachAndPatterns(cfg, &fakeAPI{matches: map[int64]opendota.Match{}}, &fakeAcquirer{}, &fakeTimelineBuilder{}, nil, patterns, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.Process(context.Background(), 48, false); err == nil {
		t.Fatal("pattern failure unexpectedly succeeded")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	match := got.Match(48)
	if match == nil || match.Status != storage.StatusCoachingReady || match.PatternRecorded {
		t.Fatalf("match state=%#v", match)
	}
	if match.LastError != "pattern disk failure" {
		t.Fatalf("last error=%q", match.LastError)
	}
}

func writeTimelineFixture(t *testing.T, root string, matchID int64) string {
	t.Helper()
	path := filepath.Join(root, "timeline.json")
	tl := timeline.MatchTimeline{
		MatchID:          matchID,
		TargetPlayerSlot: 0,
		Players: map[string]*timeline.PlayerTimeline{
			"0": {PlayerSlot: 0, HeroName: "slark"},
		},
	}
	data, err := json.Marshal(tl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
