package timeline

import (
	"math"
	"testing"
)

func TestRoshanCollectorApplyAddsLifecycleAndEnrichesKillTeams(t *testing.T) {
	gameStartTime := 100.0
	collector := &roshanCollector{states: []rawRoshanSpawnerState{
		{NetTick: 2990, AbsoluteT: 99.6666666667, Handle: invalidRoshanHandle, KillCount: 0, LastKillerTeam: -1, LastKillerTeamAvailable: true},
		{NetTick: 2995, AbsoluteT: 99.8333333333, Handle: 111, KillCount: 0, LastKillerTeam: -1, LastKillerTeamAvailable: true},
		{NetTick: 3300, AbsoluteT: 110, Handle: 111, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3302, AbsoluteT: 110.0666666667, Handle: invalidRoshanHandle, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3600, AbsoluteT: 120, Handle: 222, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3900, AbsoluteT: 130, Handle: 222, KillCount: 2, LastKillerTeam: 3, LastKillerTeamAvailable: true},
	}}

	firstKillT := 10.0
	secondKillT := 30.0
	out := MatchTimeline{Objectives: []ObjectiveEvent{
		{T: firstKillT, Type: "roshan_kill", Target: "npc_dota_roshan"},
		{T: secondKillT, Type: "roshan_kill", Target: "npc_dota_roshan"},
	}}

	collector.apply(&out, gameStartTime)

	if got := out.Objectives[0].Type; got != "roshan_alive_at_start" {
		t.Fatalf("first objective type = %q, want roshan_alive_at_start", got)
	}
	if out.Objectives[0].T != 0 {
		t.Fatalf("alive-at-start T = %v, want 0", out.Objectives[0].T)
	}

	var firstKill, secondKill *ObjectiveEvent
	var spawn *ObjectiveEvent
	for i := range out.Objectives {
		event := &out.Objectives[i]
		switch {
		case event.Type == "roshan_kill" && math.Abs(event.T-firstKillT) < 1e-9:
			firstKill = event
		case event.Type == "roshan_kill" && math.Abs(event.T-secondKillT) < 1e-9:
			secondKill = event
		case event.Type == "roshan_spawned":
			spawn = event
		}
	}
	if firstKill == nil || firstKill.AttackerTeam != 2 {
		t.Fatalf("first Roshan kill = %#v, want attacker_team 2", firstKill)
	}
	if secondKill == nil || secondKill.AttackerTeam != 3 {
		t.Fatalf("second Roshan kill = %#v, want attacker_team 3", secondKill)
	}
	if spawn == nil {
		t.Fatal("missing roshan_spawned event")
	}
	wantSpawnT := 20.0
	if math.Abs(spawn.T-wantSpawnT) > 1e-9 {
		t.Fatalf("spawn T = %v, want %v", spawn.T, wantSpawnT)
	}
}

func TestRoshanKillAlignmentUsesPauseAwareAbsoluteTime(t *testing.T) {
	gameStartTime := 246.36668395996094
	killAbsolute := pauseAwareAbsoluteTime(85568, false, 0, 1100)
	killMatchT := killAbsolute - gameStartTime
	combatLogT := 2569.199966430664

	if delta := math.Abs(killMatchT - combatLogT); delta > roshanEventSyncTolerance {
		t.Fatalf("pause-aware delta = %v, want <= %v", delta, roshanEventSyncTolerance)
	}

	collector := &roshanCollector{states: []rawRoshanSpawnerState{
		{NetTick: 4693, AbsoluteT: pauseAwareAbsoluteTime(4693, false, 0, 0), Handle: 7357729, KillCount: 0, LastKillerTeam: -1, LastKillerTeamAvailable: true},
		{NetTick: 85568, AbsoluteT: killAbsolute, Handle: 7357729, KillCount: 1, LastKillerTeam: 3, LastKillerTeamAvailable: true},
	}}
	out := MatchTimeline{Objectives: []ObjectiveEvent{{T: combatLogT, Type: "roshan_kill"}}}
	collector.apply(&out, gameStartTime)

	if out.Objectives[0].AttackerTeam != 3 {
		t.Fatalf("attacker team = %d, want 3", out.Objectives[0].AttackerTeam)
	}
}

func TestEnrichRoshanKillTeamsFailsClosedWhenTimingDoesNotMatch(t *testing.T) {
	objectives := []ObjectiveEvent{{T: 100, Type: "roshan_kill"}}
	kills := []roshanKillState{{T: 101, KillerTeam: 2, TeamKnown: true}}

	enrichRoshanKillTeams(objectives, kills)

	if objectives[0].AttackerTeam != 0 {
		t.Fatalf("attacker team = %d, want unavailable", objectives[0].AttackerTeam)
	}
}

func TestRoshanCollectorApplyDoesNothingWithoutSpawnerState(t *testing.T) {
	out := MatchTimeline{Objectives: []ObjectiveEvent{{T: 10, Type: "roshan_kill"}}}
	(&roshanCollector{}).apply(&out, 0)

	if len(out.Objectives) != 1 {
		t.Fatalf("objectives len = %d, want 1", len(out.Objectives))
	}
	if out.Objectives[0].AttackerTeam != 0 {
		t.Fatalf("attacker team = %d, want unavailable", out.Objectives[0].AttackerTeam)
	}
}
