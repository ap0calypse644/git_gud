package detector

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAssessObjectiveMissEmitsConservativeKnownRoshanCandidate(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	assessment := assessObjectiveMiss(tl, ctx)
	if !assessment.Candidate {
		t.Fatalf("assessment = %#v, want candidate", assessment)
	}
	if !assessment.Evidence.EnemyMapOpened || len(assessment.Evidence.EnemyTier1sDestroyedAtEnd) != 1 || assessment.Evidence.EnemyTier1sDestroyedAtEnd[0] != "mid" {
		t.Fatalf("map evidence = %#v", assessment.Evidence)
	}
	if assessment.Evidence.RoshanKnowledgeState != "known_alive_from_game_start" || !assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("Roshan evidence = %#v", assessment.Evidence)
	}
	if !assessment.Evidence.EnemyDeathWindowEndStateAvailable || assessment.Evidence.EnemyDeathsStillDeadAtWindowEnd != 1 {
		t.Fatalf("power-play evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesEnemyRespawnedBeforeWindowEnd(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	tl.Players["128"].Samples = append(tl.Players["128"].Samples,
		timeline.HeroSample{T: 150, Alive: true},
	)
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("respawned enemy emitted candidate: %#v", assessment)
	}
	if !assessment.Evidence.EnemyDeathWindowEndStateAvailable || assessment.Evidence.EnemyDeathsStillDeadAtWindowEnd != 0 {
		t.Fatalf("power-play evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesAmbiguousEnemyDeathState(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.EnemyDeaths = 2
	ctx.EnemyDeathAdvantage = 2
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("ambiguous death attribution emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.EnemyDeathWindowEndStateAvailable {
		t.Fatalf("ambiguous death state marked available: %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesHiddenRespawn(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.RoshanAtEnd = timeline.RoshanPostFightState{
		ReplayStateAvailable:  true,
		WorldState:            "alive",
		KnowledgeState:        "unknown_after_random_respawn",
		KnownAliveForDecision: false,
	}
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("hidden respawn emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("hidden respawn leaked into decision evidence: %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesTeamConversion(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.TargetTeamConversions = []timeline.PostFightObjectiveEvent{{
		T: 130, Type: "building_kill", Team: 2, Target: "npc_dota_badguys_tower2_mid",
	}}
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("converted window emitted miss candidate: %#v", assessment)
	}
	if assessment.Evidence.NoTargetTeamConversion || assessment.Evidence.TargetTeamConversionCount != 1 {
		t.Fatalf("conversion evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesClosedOverlapWindow(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.WindowEndT = ctx.FightObservedEndT
	ctx.WindowDurationSeconds = 0
	ctx.WindowEndReason = "overlapping_fight_active"
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("overlap window emitted candidate: %#v", assessment)
	}
}

func TestAssessObjectiveMissSuppressesBeforeEnemyMapOpened(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	for i := range ctx.EnemyLaneStructuresAtEnd {
		ctx.EnemyLaneStructuresAtEnd[i].Tier1Destroyed = false
	}
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("closed-map context emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.EnemyMapOpened {
		t.Fatalf("map evidence = %#v, want closed", assessment.Evidence)
	}
}

func baseObjectiveFixture() (*timeline.MatchTimeline, timeline.PostFightObjectiveContext) {
	enemySlot := 128
	tl := &timeline.MatchTimeline{
		Players: map[string]*timeline.PlayerTimeline{
			"0": {
				PlayerSlot: 0,
				Team:       2,
				Samples:    []timeline.HeroSample{{T: 119, Alive: true}, {T: 159, Alive: true}},
			},
			"128": {
				PlayerSlot: 128,
				Team:       3,
				Samples:    []timeline.HeroSample{{T: 119, Alive: false}, {T: 159, Alive: false}},
			},
		},
		Deaths: []timeline.DeathEvent{{T: 115, VictimSlot: &enemySlot}},
	}
	ctx := timeline.PostFightObjectiveContext{
		FightIndex:                12,
		ObservedTimingAvailable:   true,
		FightObservedStartT:       100,
		FightObservedEndT:         120,
		WindowEndT:                160,
		WindowEndReason:           "next_fight_start",
		WindowDurationSeconds:     40,
		TargetTeam:                2,
		TargetInvolved:            true,
		Participants:              []int{0, 128},
		FightDeaths:               1,
		FightHeroDamage:           6000,
		AlliedDeaths:              0,
		EnemyDeaths:               1,
		EnemyDeathAdvantage:       1,
		TargetEndSampleAvailable:  true,
		TargetEndSampleT:          119.5,
		TargetEndSampleAge:        0.5,
		TargetAliveAtEnd:          true,
		AlliedEndSamplesAvailable: 5,
		AlliedHeroesAliveAtEnd:    5,
		EnemyLaneStructuresAtEnd: []timeline.LaneStructureState{
			{Team: 3, Lane: "top", T: 120, Tier1Destroyed: false},
			{Team: 3, Lane: "mid", T: 120, Tier1Destroyed: true},
			{Team: 3, Lane: "bottom", T: 120, Tier1Destroyed: false},
		},
		RoshanAtEnd: timeline.RoshanPostFightState{
			ReplayStateAvailable:  true,
			WorldState:            "alive",
			KnowledgeState:        "known_alive_from_game_start",
			KnownAliveForDecision: true,
		},
		TargetTeamConversions: []timeline.PostFightObjectiveEvent{},
		EnemyTeamConversions:  []timeline.PostFightObjectiveEvent{},
	}
	return tl, ctx
}
