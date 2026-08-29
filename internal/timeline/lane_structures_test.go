package timeline

import "testing"

func TestDeriveLaneStructuresNormalizesLaneTowerKills(t *testing.T) {
	tl := MatchTimeline{
		LaneTowerPositions: []LaneTowerPosition{
			{Team: 2, Lane: "bottom", Tier: 1, X: 80, Y: 80, RawName: "dota_goodguys_tower1_bot"},
			{Team: 3, Lane: "mid", Tier: 2, X: 140, Y: 140, RawName: "dota_badguys_tower2_mid"},
			{Team: 3, Lane: "top", Tier: 3, X: 160, Y: 170, RawName: "dota_badguys_tower3_top"},
			{Team: 3, Lane: "base", Tier: 0, X: 171, Y: 167, RawName: "dota_badguys_fort"},
		},
		Objectives: []ObjectiveEvent{
			{T: 100, Type: "building_kill", Target: "npc_dota_goodguys_tower1_bot", TargetTeam: 2},
			{T: 200, Type: "building_kill", Target: "npc_dota_badguys_tower2_mid", TargetTeam: 3},
			{T: 300, Type: "building_kill", Target: "npc_dota_badguys_tower3_top", TargetTeam: 3},
		},
	}

	got := DeriveLaneStructures(&tl)
	if !got.Available {
		t.Fatal("expected lane structure capability available")
	}
	if got.InitialLaneTowersObserved != 3 || len(got.InitialTowers) != 3 {
		t.Fatalf("unexpected initial tower evidence: %+v", got.InitialTowers)
	}
	if got.InitialFortsObserved != 1 || len(got.InitialForts) != 1 || got.InitialForts[0].Team != 3 || got.InitialForts[0].X != 171 || got.InitialForts[0].Y != 167 {
		t.Fatalf("unexpected initial fort evidence: %+v", got.InitialForts)
	}
	if got.BuildingKillsObserved != 3 || got.LaneTowerKillsAccepted != 3 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	want := []LaneStructureEvent{
		{T: 100, Team: 2, Lane: "bottom", Tier: 1, RawTarget: "npc_dota_goodguys_tower1_bot"},
		{T: 200, Team: 3, Lane: "mid", Tier: 2, RawTarget: "npc_dota_badguys_tower2_mid"},
		{T: 300, Team: 3, Lane: "top", Tier: 3, RawTarget: "npc_dota_badguys_tower3_top"},
	}
	if len(got.Events) != len(want) {
		t.Fatalf("events = %d, want %d", len(got.Events), len(want))
	}
	for i := range want {
		if got.Events[i] != want[i] {
			t.Fatalf("event[%d] = %+v, want %+v", i, got.Events[i], want[i])
		}
	}
}

func TestDeriveLaneStructuresIgnoresNonLaneStructures(t *testing.T) {
	tl := MatchTimeline{Objectives: []ObjectiveEvent{
		{T: 10, Type: "building_kill", Target: "npc_dota_goodguys_tower4", TargetTeam: 2},
		{T: 20, Type: "building_kill", Target: "npc_dota_goodguys_melee_rax_bot", TargetTeam: 2},
		{T: 30, Type: "building_kill", Target: "npc_dota_goodguys_fort", TargetTeam: 2},
		{T: 40, Type: "building_kill", Target: "npc_dota_goodguys_fillers", TargetTeam: 2},
	}}

	got := DeriveLaneStructures(&tl)
	if got.BuildingKillsObserved != 4 || got.IgnoredNonLaneStructures != 4 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if len(got.Events) != 0 {
		t.Fatalf("events = %d, want 0", len(got.Events))
	}
}

func TestDeriveLaneStructuresRejectsMalformedOrTeamMismatch(t *testing.T) {
	tl := MatchTimeline{Objectives: []ObjectiveEvent{
		{T: 10, Type: "building_kill", Target: "npc_dota_goodguys_tower9_bot", TargetTeam: 2},
		{T: 20, Type: "building_kill", Target: "npc_dota_goodguys_tower1_bot", TargetTeam: 3},
	}}

	got := DeriveLaneStructures(&tl)
	if got.RejectedMalformed != 2 {
		t.Fatalf("rejected malformed = %d, want 2", got.RejectedMalformed)
	}
	if len(got.Events) != 0 {
		t.Fatalf("events = %d, want 0", len(got.Events))
	}
}

func TestDeriveLaneStructuresDeduplicatesEarliestDestruction(t *testing.T) {
	tl := MatchTimeline{Objectives: []ObjectiveEvent{
		{T: 100, Type: "building_kill", Target: "npc_dota_badguys_tower1_top", TargetTeam: 3},
		{T: 110, Type: "building_kill", Target: "npc_dota_badguys_tower1_top", TargetTeam: 3},
	}}
	got := DeriveLaneStructures(&tl)
	if len(got.Events) != 1 || got.Events[0].T != 100 {
		t.Fatalf("unexpected dedup result: %+v", got.Events)
	}
}

func TestLaneStructureStateAtIsCausal(t *testing.T) {
	timeline := LaneStructureTimeline{
		Available: true,
		InitialTowers: []LaneStructureInitialTower{
			{Team: 3, Lane: "bottom", Tier: 1},
			{Team: 3, Lane: "bottom", Tier: 2},
			{Team: 3, Lane: "bottom", Tier: 3},
		},
		Events: []LaneStructureEvent{
			{T: 100, Team: 3, Lane: "bottom", Tier: 1},
			{T: 200, Team: 3, Lane: "bottom", Tier: 2},
			{T: 300, Team: 3, Lane: "bottom", Tier: 3},
		},
	}

	before, ok := LaneStructureStateAt(timeline, 3, "bot", 50)
	if !ok {
		t.Fatal("expected valid point-in-time state")
	}
	if !before.Tier1KnownAlive || !before.Tier2KnownAlive || !before.Tier3KnownAlive {
		t.Fatalf("initial known-alive state missing: %+v", before)
	}

	middle, ok := LaneStructureStateAt(timeline, 3, "bot", 150)
	if !ok {
		t.Fatal("expected valid point-in-time state")
	}
	if !middle.Tier1Destroyed || middle.Tier1KnownAlive || middle.Tier2Destroyed || middle.Tier3Destroyed {
		t.Fatalf("future destruction leaked: %+v", middle)
	}
	if !middle.Tier2KnownAlive || !middle.Tier3KnownAlive {
		t.Fatalf("surviving tower existence lost: %+v", middle)
	}
	if middle.Tier1DestroyedAt == nil || *middle.Tier1DestroyedAt != 100 {
		t.Fatalf("unexpected tier1 destruction time: %+v", middle)
	}

	after, ok := LaneStructureStateAt(timeline, 3, "bottom", 350)
	if !ok || !after.Tier1Destroyed || !after.Tier2Destroyed || !after.Tier3Destroyed {
		t.Fatalf("unexpected final state: %+v ok=%v", after, ok)
	}
	if after.Tier1KnownAlive || after.Tier2KnownAlive || after.Tier3KnownAlive {
		t.Fatalf("destroyed towers remained known alive: %+v", after)
	}
}

func TestLaneStructureStateAtDoesNotInferAliveWithoutInitialEntity(t *testing.T) {
	timeline := LaneStructureTimeline{Available: true}
	state, ok := LaneStructureStateAt(timeline, 3, "bottom", 10)
	if !ok {
		t.Fatal("expected valid point-in-time state")
	}
	if state.Tier1KnownAlive || state.Tier2KnownAlive || state.Tier3KnownAlive {
		t.Fatalf("absence of destruction incorrectly promoted to alive: %+v", state)
	}
}

func TestLaneStructureStateAtFailsClosed(t *testing.T) {
	if _, ok := LaneStructureStateAt(LaneStructureTimeline{}, 3, "bottom", 10); ok {
		t.Fatal("unavailable timeline should fail closed")
	}
	if _, ok := LaneStructureStateAt(LaneStructureTimeline{Available: true}, 4, "bottom", 10); ok {
		t.Fatal("invalid team should fail closed")
	}
	if _, ok := LaneStructureStateAt(LaneStructureTimeline{Available: true}, 3, "jungle", 10); ok {
		t.Fatal("invalid lane should fail closed")
	}
}
