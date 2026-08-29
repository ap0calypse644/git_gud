package timeline

import (
	"math"
	"testing"
)

func TestRoshanCollectorApplyAddsLifecycleAndEnrichesKillTeams(t *testing.T) {
	gameStartTime := 100.0
	collector := &roshanCollector{states: []rawRoshanSpawnerState{
		{NetTick: 2990, Handle: invalidRoshanHandle, KillCount: 0, LastKillerTeam: -1, LastKillerTeamAvailable: true},
		{NetTick: 2995, Handle: 111, KillCount: 0, LastKillerTeam: -1, LastKillerTeamAvailable: true},
		{NetTick: 3300, Handle: 111, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3302, Handle: invalidRoshanHandle, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3600, Handle: 222, KillCount: 1, LastKillerTeam: 2, LastKillerTeamAvailable: true},
		{NetTick: 3900, Handle: 222, KillCount: 2, LastKillerTeam: 3, LastKillerTeamAvailable: true},
	}}

	firstKillT := float64(3300)/tickRate - gameStartTime
	secondKillT := float64(3900)/tickRate - gameStartTime
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
	wantSpawnT := float64(3600)/tickRate - gameStartTime
	if math.Abs(spawn.T-wantSpawnT) > 1e-9 {
		t.Fatalf("spawn T = %v, want %v", spawn.T, wantSpawnT)
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
