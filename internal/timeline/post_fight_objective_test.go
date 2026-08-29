package timeline

import "testing"

func TestDerivePostFightObjectiveTimelineUsesNextFightBoundary(t *testing.T) {
	enemySlot := 128
	tl := MatchTimeline{
		DurationSeconds: 80,
		TargetPlayerSlot: 0,
		Players: map[string]*PlayerTimeline{
			"0": {
				PlayerSlot: 0,
				Team:       2,
				Samples: []HeroSample{
					{T: 19, Alive: true},
					{T: 49, Alive: true},
				},
			},
			"1": {
				PlayerSlot: 1,
				Team:       2,
				Samples: []HeroSample{
					{T: 19, Alive: true},
					{T: 49, Alive: true},
				},
			},
			"128": {
				PlayerSlot: 128,
				Team:       3,
				Samples: []HeroSample{{T: 18, Alive: false}},
			},
		},
		Fights: []FightWindow{
			{ObservedStartT: 10, ObservedEndT: 20, Participants: []int{0, 1, 128}, Deaths: 1, HeroDamage: 2000},
			{ObservedStartT: 40, ObservedEndT: 50, Participants: []int{0, 1}, HeroDamage: 500},
		},
		Deaths: []DeathEvent{{T: 18, VictimSlot: &enemySlot}},
		Objectives: []ObjectiveEvent{
			{T: 0, Type: "roshan_alive_at_start", Target: "npc_dota_roshan"},
			{T: 30, Type: "building_kill", AttackerTeam: 2, TargetTeam: 3, Target: "npc_dota_badguys_tower1_mid"},
			{T: 40, Type: "building_kill", AttackerTeam: 2, TargetTeam: 3, Target: "npc_dota_badguys_tower2_mid"},
		},
		LaneStructures: LaneStructureTimeline{Available: true, Events: []LaneStructureEvent{}},
	}

	out := DerivePostFightObjectiveTimeline(&tl)
	if !out.Available || len(out.Contexts) != 2 {
		t.Fatalf("timeline = %#v, want two available contexts", out)
	}

	first := out.Contexts[0]
	if first.WindowEndReason != "next_fight_start" || first.WindowEndT != 40 || first.WindowDurationSeconds != 20 {
		t.Fatalf("first window = %#v, want 20..40 next-fight window", first)
	}
	if first.EnemyDeaths != 1 || first.AlliedDeaths != 0 || first.EnemyDeathAdvantage != 1 {
		t.Fatalf("first death evidence = allied %d enemy %d advantage %d", first.AlliedDeaths, first.EnemyDeaths, first.EnemyDeathAdvantage)
	}
	if !first.TargetEndSampleAvailable || !first.TargetAliveAtEnd || first.AlliedHeroesAliveAtEnd != 2 {
		t.Fatalf("first allied end state = %#v", first)
	}
	if !first.RoshanAtEnd.KnownAliveForDecision || first.RoshanAtEnd.KnowledgeState != "known_alive_from_game_start" {
		t.Fatalf("first Roshan state = %#v, want known alive", first.RoshanAtEnd)
	}
	if len(first.TargetTeamConversions) != 1 || first.TargetTeamConversions[0].T != 30 {
		t.Fatalf("first conversions = %#v, want only t=30", first.TargetTeamConversions)
	}

	second := out.Contexts[1]
	if second.WindowEndReason != "match_end" || second.WindowEndT != 80 {
		t.Fatalf("second window = %#v, want match-end boundary", second)
	}
}

func TestRoshanPostFightStateDoesNotPromoteHiddenRespawnToKnowledge(t *testing.T) {
	objectives := []ObjectiveEvent{
		{T: 0, Type: "roshan_alive_at_start"},
		{T: 100, Type: "roshan_kill", AttackerTeam: 2},
		{T: 200, Type: "roshan_spawned"},
	}

	dead := roshanPostFightStateAt(objectives, 150)
	if dead.WorldState != "dead" || dead.KnowledgeState != "known_dead_from_kill" || dead.KnownAliveForDecision {
		t.Fatalf("state at 150 = %#v, want known dead", dead)
	}
	if dead.LastSpawnT != nil {
		t.Fatalf("state at 150 leaked future respawn: %#v", dead)
	}

	respawned := roshanPostFightStateAt(objectives, 220)
	if respawned.WorldState != "alive" || respawned.KnowledgeState != "unknown_after_random_respawn" {
		t.Fatalf("state at 220 = %#v, want replay-alive but knowledge-unknown", respawned)
	}
	if respawned.KnownAliveForDecision {
		t.Fatalf("hidden respawn became decision knowledge: %#v", respawned)
	}
	if respawned.LastSpawnT == nil || *respawned.LastSpawnT != 200 {
		t.Fatalf("last spawn = %#v, want replay fact t=200", respawned.LastSpawnT)
	}
}

func TestRoshanPostFightStateDoesNotReadFutureKill(t *testing.T) {
	objectives := []ObjectiveEvent{
		{T: 0, Type: "roshan_alive_at_start"},
		{T: 300, Type: "roshan_kill", AttackerTeam: 3},
	}
	state := roshanPostFightStateAt(objectives, 250)
	if state.WorldState != "alive" || state.KnowledgeState != "known_alive_from_game_start" || !state.KnownAliveForDecision {
		t.Fatalf("state at 250 = %#v, want causal known-alive state", state)
	}
	if state.LastKillT != nil || state.LastKillerTeam != 0 {
		t.Fatalf("state at 250 leaked future kill: %#v", state)
	}
}
