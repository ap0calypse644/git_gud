package processor

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/storage"
)

func TestProcessForceRegeneratesCoachingReadyFromTimeline(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 49)
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 100
	state.Put(&storage.MatchState{
		MatchID:         49,
		Status:          storage.StatusCoachingReady,
		TimelinePath:    timelinePath,
		ReportPath:      filepath.Join(root, "reports", "old-49.json"),
		PatternRecorded: true,
	})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{matches: map[int64]opendota.Match{}}
	acquirer := &fakeAcquirer{}
	timelineBuilder := &fakeTimelineBuilder{}
	coach := &fakeCoach{path: filepath.Join(root, "reports", "new-49.json")}
	svc := NewWithCoach(cfg, api, acquirer, timelineBuilder, coach, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 49, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusCoachingReady || result.ReportPath != coach.path {
		t.Fatalf("result=%#v", result)
	}
	if len(coach.calls) != 1 || coach.calls[0].MatchID != 49 || coach.calls[0].Hero != "slark" {
		t.Fatalf("coach calls=%#v", coach.calls)
	}
	if len(api.matchCalls) != 0 || len(acquirer.calls) != 0 || len(timelineBuilder.calls) != 0 {
		t.Fatalf("manual reanalysis repeated upstream work: api=%v acquire=%v timeline=%v", api.matchCalls, acquirer.calls, timelineBuilder.calls)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSeenMatchID != 100 {
		t.Fatalf("manual reanalysis changed watcher baseline: got %d want 100", got.LastSeenMatchID)
	}
	match := got.Match(49)
	if match == nil || match.Status != storage.StatusCoachingReady || match.ReportPath != coach.path {
		t.Fatalf("match state=%#v", match)
	}
}

func TestProcessCoachingReadyDoesNotRegenerateWithoutForce(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	store := storage.New(filepath.Join(root, "state.json"))
	timelinePath := writeTimelineFixture(t, root, 50)
	oldReport := filepath.Join(root, "reports", "old-50.json")
	state := storage.NewState()
	state.Put(&storage.MatchState{
		MatchID:         50,
		Status:          storage.StatusCoachingReady,
		TimelinePath:    timelinePath,
		ReportPath:      oldReport,
		PatternRecorded: true,
	})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	coach := &fakeCoach{path: filepath.Join(root, "reports", "new-50.json")}
	svc := NewWithCoach(cfg, &fakeAPI{matches: map[int64]opendota.Match{}}, &fakeAcquirer{}, &fakeTimelineBuilder{}, coach, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := svc.Process(context.Background(), 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != storage.StatusCoachingReady || result.ReportPath != oldReport {
		t.Fatalf("result=%#v", result)
	}
	if len(coach.calls) != 0 {
		t.Fatalf("automatic processing regenerated coaching: calls=%#v", coach.calls)
	}
}
