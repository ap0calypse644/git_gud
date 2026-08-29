package detector

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAssessObjectiveMissEmitsConservativeKnownObjectiveCandidate(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	assessment := assessObjectiveMiss(tl, ctx)
	if !assessment.Candidate {
		t.Fatalf("assessment = %#v, want candidate", assessment)
	}
	if !assessment.Evidence.EnemyMapOpened || len(assessment.Evidence.EnemyTier1sDestroyedAtEnd) != 1 || assessment.Evidence.EnemyTier1sDestroyedAtEnd[0] != "mid" {
		t.Fatalf("map evidence = %#v", assessment.Evidence)
	}
	if len(assessment.Evidence.EnemyFrontTowerOptions) != 3 {
		t.Fatalf("front tower options = %#v, want 3", assessment.Evidence.EnemyFrontTowerOptions)
	}
	if assessment.Evidence.EnemyFrontTowerOptions[0] != (ObjectiveTowerOption{Lane: "bottom", Tier: 1}) ||
		assessment.Evidence.EnemyFrontTowerOptions[1] != (ObjectiveTowerOption{Lane: "mid", Tier: 2}) ||
		assessment.Evidence.EnemyFrontTowerOptions[2] != (ObjectiveTowerOption{Lane: "top", Tier: 1}) {
		t.Fatalf("unexpected front tower options = %#v", assessment.Evidence.EnemyFrontTowerOptions)
	}
	if assessment.Evidence.RoshanKnowledgeState != "known_alive_from_game_start" || !assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("Roshan evidence = %#v", assessment.Evidence)
	}
	if assessment.Evidence.KnownObjectiveOptionCount != 4 || !assessment.Evidence.KnownObjectiveOptions {
		t.Fatalf("objective option evidence = %#v", assessment.Evidence)
	}
	if !assessment.Evidence.EnemyDeathWindowEndStateAvailable || assessment.Evidence.EnemyDeathsStillDeadAtWindowEnd != 1 {
		t.Fatalf("power-play evidence = %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesEnemyRespawnedBeforeWindowEnd(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	tl.Players["128"].Samples[1].Alive = true
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

func TestAssessObjectiveMissDoesNotLeakHiddenRoshanRespawnWhenTowerOptionExists(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.RoshanAtEnd = timeline.RoshanPostFightState{
		ReplayStateAvailable:  true,
		WorldState:            "alive",
		KnowledgeState:        "unknown_after_random_respawn",
		KnownAliveForDecision: false,
	}
	assessment := assessObjectiveMiss(tl, ctx)
	if !assessment.Candidate {
		t.Fatalf("known tower options should still support candidate: %#v", assessment)
	}
	if assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("hidden respawn leaked into decision evidence: %#v", assessment.Evidence)
	}
	if assessment.Evidence.KnownObjectiveOptionCount != 3 {
		t.Fatalf("hidden Roshan should not count as objective option: %#v", assessment.Evidence)
	}
}

func TestAssessObjectiveMissSuppressesWhenNoObjectiveKnownAvailable(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.RoshanAtEnd = timeline.RoshanPostFightState{
		ReplayStateAvailable:  true,
		WorldState:            "alive",
		KnowledgeState:        "unknown_after_random_respawn",
		KnownAliveForDecision: false,
	}
	for i := range ctx.EnemyLaneStructuresAtEnd {
		ctx.EnemyLaneStructuresAtEnd[i].Tier1KnownAlive = false
		ctx.EnemyLaneStructuresAtEnd[i].Tier2KnownAlive = false
		ctx.EnemyLaneStructuresAtEnd[i].Tier3KnownAlive = false
	}
	assessment := assessObjectiveMiss(tl, ctx)
	if assessment.Candidate {
		t.Fatalf("unknown objective availability emitted candidate: %#v", assessment)
	}
	if assessment.Evidence.KnownObjectiveOptions || assessment.Evidence.KnownObjectiveOptionCount != 0 {
		t.Fatalf("objective availability = %#v, want none", assessment.Evidence)
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

func TestObjectiveFrontTowerRequiresCausalLaneProgression(t *testing.T) {
	if got, ok := objectiveFrontTower(timeline.LaneStructureState{Lane: "mid", Tier2KnownAlive: true}); ok {
		t.Fatalf("unexposed T2 accepted: %#v", got)
	}
	if got, ok := objectiveFrontTower(timeline.LaneStructureState{Lane: "mid", Tier1Destroyed: true, Tier2KnownAlive: true}); !ok || got.Tier != 2 {
		t.Fatalf("exposed T2 = %#v ok=%v, want tier 2", got, ok)
	}
	if got, ok := objectiveFrontTower(timeline.LaneStructureState{Lane: "mid", Tier1Destroyed: true, Tier2Destroyed: true, Tier3KnownAlive: true}); !ok || got.Tier != 3 {
		t.Fatalf("exposed T3 = %#v ok=%v, want tier 3", got, ok)
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
			{
				Team: 3, Lane: "top", T: 120,
				Tier1PresentAtStart: true, Tier2PresentAtStart: true, Tier3PresentAtStart: true,
				Tier1KnownAlive: true, Tier2KnownAlive: true, Tier3KnownAlive: true,
			},
			{
				Team: 3, Lane: "mid", T: 120,
				Tier1PresentAtStart: true, Tier2PresentAtStart: true, Tier3PresentAtStart: true,
				Tier1Destroyed: true, Tier1DestroyedAt: float64Ptr(90),
				Tier2KnownAlive: true, Tier3KnownAlive: true,
			},
			{
				Team: 3, Lane: "bottom", T: 120,
				Tier1PresentAtStart: true, Tier2PresentAtStart: true, Tier3PresentAtStart: true,
				Tier1KnownAlive: true, Tier2KnownAlive: true, Tier3KnownAlive: true,
			},
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

func float64Ptr(v float64) *float64 { return &v }
