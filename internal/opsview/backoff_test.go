package opsview

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ap0calypse644/git_gud/internal/storage"
)

func TestBuildStatusPrefersPersistedBackoffSchedule(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	lastAttempt := now.Add(-30 * time.Minute)
	nextRetry := now.Add(40 * time.Minute)
	state := storage.NewState()
	state.Put(&storage.MatchState{
		MatchID:       200,
		Status:        storage.StatusReplayWaiting,
		LastAttemptAt: &lastAttempt,
		RetryCount:    4,
		NextRetryAt:   &nextRetry,
	})

	snapshot := BuildStatus(state, 5*time.Minute, true, true, now)
	if len(snapshot.Pending) != 1 {
		t.Fatalf("pending=%#v", snapshot.Pending)
	}
	pending := snapshot.Pending[0]
	if pending.RetryCount != 4 || pending.NextRetryAt == nil || !pending.NextRetryAt.Equal(nextRetry) {
		t.Fatalf("pending=%#v", pending)
	}

	var out bytes.Buffer
	if err := WriteStatus(&out, snapshot); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"retry=2026-08-30T12:40:00Z", "retry_count=4"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status missing %q:\n%s", want, out.String())
		}
	}
}
