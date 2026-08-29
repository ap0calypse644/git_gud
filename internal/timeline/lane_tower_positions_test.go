package timeline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFinalizeLaneTowerPositionsDeterministicOrder(t *testing.T) {
	observed := map[[3]int]LaneTowerPosition{
		{3, laneOrder("bottom"), 2}: {Team: 3, Lane: "bottom", Tier: 2},
		{2, laneOrder("mid"), 1}:    {Team: 2, Lane: "mid", Tier: 1},
		{2, laneOrder("bottom"), 3}: {Team: 2, Lane: "bottom", Tier: 3},
		{2, laneOrder("bottom"), 1}: {Team: 2, Lane: "bottom", Tier: 1},
	}

	got := finalizeLaneTowerPositions(observed)
	if len(got) != 4 {
		t.Fatalf("positions = %d, want 4", len(got))
	}
	want := []struct {
		team int
		lane string
		tier int
	}{
		{2, "mid", 1},
		{2, "bottom", 1},
		{2, "bottom", 3},
		{3, "bottom", 2},
	}
	for i, expected := range want {
		if got[i].Team != expected.team || got[i].Lane != expected.lane || got[i].Tier != expected.tier {
			t.Fatalf("position[%d] = %+v, want team=%d lane=%s tier=%d", i, got[i], expected.team, expected.lane, expected.tier)
		}
	}
}

func TestMatchTimelineDoesNotSerializeInternalLaneTowerPositions(t *testing.T) {
	timeline := MatchTimeline{
		LaneTowerPositions: []LaneTowerPosition{{Team: 2, Lane: "mid", Tier: 1, X: 100, Y: 100}},
	}
	encoded, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	if strings.Contains(string(encoded), "lane_tower_positions") || strings.Contains(string(encoded), `"raw_name"`) {
		t.Fatalf("internal tower geometry leaked into timeline JSON: %s", encoded)
	}
}
