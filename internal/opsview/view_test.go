package opsview

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

func TestBuildStatusTracksPendingRetryAndPatternBackfill(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	lastAttempt := now.Add(-2 * time.Minute)
	state := storage.NewState()
	state.Initialized = true
	state.LastSeenMatchID = 200
	state.Put(&storage.MatchState{MatchID: 100, Status: storage.StatusReplayWaiting, LastAttemptAt: &lastAttempt, LastError: "waiting"})
	state.Put(&storage.MatchState{MatchID: 101, Status: storage.StatusCoachingReady, PatternRecorded: false})
	state.Put(&storage.MatchState{MatchID: 102, Status: storage.StatusCoachingReady, PatternRecorded: true})
	state.Put(&storage.MatchState{MatchID: 103, Status: storage.StatusReplayUnavailable})

	snapshot := BuildStatus(state, 5*time.Minute, true, true, now)
	if snapshot.PendingMatches != 2 {
		t.Fatalf("pending=%d", snapshot.PendingMatches)
	}
	if snapshot.PatternRecorded != 1 {
		t.Fatalf("pattern recorded=%d", snapshot.PatternRecorded)
	}
	if len(snapshot.Pending) != 2 || snapshot.Pending[0].MatchID != 100 || snapshot.Pending[1].MatchID != 101 {
		t.Fatalf("pending=%#v", snapshot.Pending)
	}
	if snapshot.Pending[0].NextRetryAt == nil || !snapshot.Pending[0].NextRetryAt.Equal(lastAttempt.Add(5*time.Minute)) {
		t.Fatalf("retry=%v", snapshot.Pending[0].NextRetryAt)
	}
	if snapshot.Pending[1].NextRetryAt != nil {
		t.Fatalf("backfill retry=%v", snapshot.Pending[1].NextRetryAt)
	}
}

func TestRecentHistoryReadsReportSummaryAndSortsNewestFirst(t *testing.T) {
	root := t.TempDir()
	reportPath := filepath.Join(root, "report.json")
	report := coaching.MatchCoachingReport{MatchID: 11, Hero: "slark", Summary: "Review isolated deaths."}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	state := storage.NewState()
	state.Put(&storage.MatchState{MatchID: 10, StartTime: 100, Status: storage.StatusTimelineReady})
	state.Put(&storage.MatchState{MatchID: 11, StartTime: 200, Status: storage.StatusCoachingReady, ReportPath: reportPath, PatternRecorded: true})
	entries, err := RecentHistory(state, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].MatchID != 11 || entries[0].Hero != "slark" || entries[0].Summary != "Review isolated deaths." {
		t.Fatalf("entries=%#v", entries)
	}
}

func TestWriteStatusAndHistoryExposeUsefulOperationalFields(t *testing.T) {
	snapshot := StatusSnapshot{
		Initialized:     true,
		LastSeenMatchID: 77,
		TrackedMatches:  2,
		PendingMatches:  1,
		PatternRecorded: 1,
		StatusCounts: map[storage.MatchStatus]int{
			storage.StatusCoachingReady: 1,
			storage.StatusReplayWaiting: 1,
		},
		Pending: []PendingMatch{{MatchID: 76, Status: storage.StatusReplayWaiting, LastError: "not ready"}},
	}
	var status bytes.Buffer
	if err := WriteStatus(&status, snapshot); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"watcher_initialized: true", "pending_matches: 1", "status.coaching_ready: 1", "pending.76:"} {
		if !strings.Contains(status.String(), want) {
			t.Fatalf("status missing %q:\n%s", want, status.String())
		}
	}

	var history bytes.Buffer
	if err := WriteHistory(&history, []HistoryEntry{{MatchID: 77, Status: storage.StatusCoachingReady, Hero: "slark", PatternRecorded: true, ReportPath: "data/reports/77.json", Summary: "One\nline summary"}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MATCH_ID", "slark", "data/reports/77.json", "summary: One line summary"} {
		if !strings.Contains(history.String(), want) {
			t.Fatalf("history missing %q:\n%s", want, history.String())
		}
	}
}
