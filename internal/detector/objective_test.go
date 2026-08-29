package detector

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAssessObjectiveMissEmitsConservativeKnownRoshanCandidate(t *testing.T) {
	ctx := baseObjectiveContext()
	assessment := assessObjectiveMiss(ctx)
	if !assessment.Candidate {
		t.Fatalf("assessment = %#v, want candidate", assessment)
	}
	if !assessment.Evidence.EnemyMapOpened || len(assessment.Evidence.EnemyTier1sDestroyedAtEnd) != 1 || assessment.Evidence.EnemyTier1sDestroyedAtEnd[0] != "mid" {
		t.Fatalf("map evidence = %#v", assessment.Evidence)
	}
	if assessment.Evidence.RoshanKnowledgeState != "known_alive_from_game_start" || !assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("Roshan evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesHiddenRespawn(t *testing.T) {
	ctx := baseObjectiveContext()
	ctx.RoshanAtEnd = timeline.RoshanPostFightState{
		ReplayStateAvailable:  true,
		WorldState:            "alive",
		KnowledgeState:        "unknown_after_random_respawn",
		KnownAliveForDecision: false,
	}
	assessment := assessObjectiveMiss(ctx)
	if assessment.Candidate {
		t.Fatalf("hidden respawn emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("hidden respawn leaked into decision evidence: %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesTeamConversion(t *testing.T) {
	ctx := baseObjectiveContext()
	ctx.TargetTeamConversions = []timeline.PostFightObjectiveEvent{{
		T: 130, Type: "building_kill", Team: 2, Target: "npc_dota_badguys_tower2_mid",
	}}
	assessment := assessObjectiveMiss(ctx)
	if assessment.Candidate {
		t.Fatalf("converted window emitted miss candidate: %#v", assessment)
	}
	if assessment.Evidence.NoTargetTeamConversion || assessment.Evidence.TargetTeamConversionCount != 1 {
		t.Fatalf("conversion evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesClosedOverlapWindow(t *testing.T) {
	ctx := baseObjectiveContext()
	ctx.WindowEndT = ctx.FightObservedEndT
	ctx.WindowDurationSeconds = 0
	ctx.WindowEndReason = "overlapping_fight_active"
	assessment := assessObjectiveMiss(ctx)
	if assessment.Candidate {
		t.Fatalf("overlap window emitted candidate: %#v", assessment)
	}
}

func TestAssessObjectiveMissSuppressesBeforeEnemyMapOpened(t *testing.T) {
	ctx := baseObjectiveContext()
	for i := range ctx.EnemyLaneStructuresAtEnd {
		ctx.EnemyLaneStructuresAtEnd[i].Tier1Destroyed = false
	}
	assessment := assessObjectiveMiss(ctx)
	if assessment.Candidate {
		t.Fatalf("closed-map context emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.EnemyMapOpened {
		t.Fatalf("map evidence = %#v, want closed", assessment.Evidence)
	}
}

func baseObjectiveContext() timeline.PostFightObjectiveContext {
	return timeline.PostFightObjectiveContext{
		FightIndex:                12,
		ObservedTimingAvailable:   true,
		FightObservedStartT:       100,
		FightObservedEndT:         120,
		WindowEndT:                160,
		WindowEndReason:           "next_fight_start",
		WindowDurationSeconds:     40,
		TargetTeam:                2,
		TargetInvolved:            true,
		FightDeaths:               2,
		FightHeroDamage:           6000,
		AlliedDeaths:              0,
		EnemyDeaths:               2,
		EnemyDeathAdvantage:       2,
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
}
