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
		{2, -1, 0}:                  {Team: 2, Lane: "base", Tier: 0},
	}

	got := finalizeLaneTowerPositions(observed)
	if len(got) != 5 {
		t.Fatalf("positions = %d, want 5", len(got))
	}
	want := []struct {
		team int
		lane string
		tier int
	}{
		{2, "mid", 1},
		{2, "bottom", 1},
		{2, "bottom", 3},
		{2, "base", 0},
		{3, "bottom", 2},
	}
	for i, expected := range want {
		if got[i].Team != expected.team || got[i].Lane != expected.lane || got[i].Tier != expected.tier {
			t.Fatalf("position[%d] = %+v, want team=%d lane=%s tier=%d", i, got[i], expected.team, expected.lane, expected.tier)
		}
	}
}

func TestParseFortEntityName(t *testing.T) {
	for name, wantTeam := range map[string]int{
		"dota_goodguys_fort":     2,
		"npc_dota_goodguys_fort": 2,
		"dota_badguys_fort":      3,
		"npc_dota_badguys_fort":  3,
	} {
		team, ok := parseFortEntityName(name)
		if !ok || team != wantTeam {
			t.Fatalf("parseFortEntityName(%q) = %d,%v want %d,true", name, team, ok, wantTeam)
		}
	}
	if _, ok := parseFortEntityName("dota_goodguys_tower4"); ok {
		t.Fatal("non-Fort entity accepted as Fort")
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
