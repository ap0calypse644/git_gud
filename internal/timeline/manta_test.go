package timeline

import "testing"

func TestMatchPlayerSlot(t *testing.T) {
	tests := []struct {
		playerID int
		team     int
		want     int
		ok       bool
	}{
		{playerID: 0, team: 2, want: 0, ok: true},
		{playerID: 4, team: 2, want: 4, ok: true},
		{playerID: 5, team: 3, want: 128, ok: true},
		{playerID: 9, team: 3, want: 132, ok: true},
		{playerID: 2, team: 3, want: 130, ok: true},
		{playerID: 8, team: 2, ok: false},
	}
	for _, tc := range tests {
		got, ok := matchPlayerSlot(tc.playerID, tc.team)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("matchPlayerSlot(%d, %d) = (%d, %v), want (%d, %v)", tc.playerID, tc.team, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNumberHelpers(t *testing.T) {
	if got, ok := numberInt(uint32(42)); !ok || got != 42 {
		t.Fatalf("numberInt = (%d, %v)", got, ok)
	}
	if got, ok := numberUint(int64(99)); !ok || got != 99 {
		t.Fatalf("numberUint = (%d, %v)", got, ok)
	}
	if _, ok := numberUint(int32(-1)); ok {
		t.Fatal("numberUint accepted negative value")
	}
	if got, ok := numberFloat(float32(12.5)); !ok || got != 12.5 {
		t.Fatalf("numberFloat = (%f, %v)", got, ok)
	}
}
