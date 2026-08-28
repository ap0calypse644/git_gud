package storage

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "nested", "state.json"))
	state := NewState()
	state.Initialized = true
	state.LastSeenMatchID = 42
	state.Put(&MatchState{MatchID: 42, Status: StatusReplayWaiting})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Initialized || got.LastSeenMatchID != 42 || got.Match(42) == nil {
		t.Fatalf("state = %#v", got)
	}
}
