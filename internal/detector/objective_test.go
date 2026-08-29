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
	if len(assessment.Evidence.EnemyPushableTowerOptions) != 2 ||
		assessment.Evidence.EnemyPushableTowerOptions[0] != (ObjectiveTowerOption{Lane: "bottom", Tier: 1}) ||
		assessment.Evidence.EnemyPushableTowerOptions[1] != (ObjectiveTowerOption{Lane: "top", Tier: 1}) {
		t.Fatalf("pushable tower options = %#v", assessment.Evidence.EnemyPushableTowerOptions)
	}
	if assessment.Evidence.RoshanKnowledgeState != "known_alive_from_game_start" || !assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("Roshan evidence = %#v", assessment.Evidence)
	}
	if assessment.Evidence.KnownObjectiveOptionCount != 3 || !assessment.Evidence.KnownObjectiveOptions {
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
		t.Fatalf("unprotected T1 options should still support candidate: %#v", assessment)
	}
	if assessment.Evidence.RoshanKnownAliveForDecision {
		t.Fatalf("hidden respawn leaked into decision evidence: %#v", assessment.Evidence)
	}
	if assessment.Evidence.KnownObjectiveOptionCount != 2 {
		t.Fatalf("hidden Roshan/T2 without creep support should not count: %#v", assessment.Evidence)
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

func TestAssessObjectiveMissRequiresCreepSupportForTier2(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.RoshanAtEnd.KnownAliveForDecision = false
	ctx.RoshanAtEnd.KnowledgeState = "known_dead_from_kill"
	for i := range ctx.EnemyLaneStructuresAtEnd {
		state := &ctx.EnemyLaneStructuresAtEnd[i]
		if state.Lane == "mid" {
			continue
		}
		state.Tier1KnownAlive = false
		state.Tier2KnownAlive = false
		state.Tier3KnownAlive = false
	}

	withoutSupport := assessObjectiveMiss(tl, ctx)
	if withoutSupport.Candidate {
		t.Fatalf("unsupported T2 emitted candidate: %#v", withoutSupport)
	}

	tl.CreepClusters.Frames = []timeline.CreepClusterFrame{{
		T: 125,
		Clusters: []timeline.CreepCluster{{
			Team: 2, CenterX: 150, CenterY: 150, CreepCount: 4, LaneCreepCount: 4,
			MaxMemberDistanceWorld: 100,
		}},
	}}
	withSupport := assessObjectiveMiss(tl, ctx)
	if !withSupport.Candidate {
		t.Fatalf("supported T2 suppressed: %#v", withSupport)
	}
	if len(withSupport.Evidence.EnemyPushableTowerOptions) != 1 || withSupport.Evidence.EnemyPushableTowerOptions[0] != (ObjectiveTowerOption{Lane: "mid", Tier: 2}) {
		t.Fatalf("T2 pushable evidence = %#v", withSupport.Evidence.EnemyPushableTowerOptions)
	}
}

func TestAssessObjectiveMissRequiresAncientCreepSupportForTier3(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	ctx.RoshanAtEnd.KnownAliveForDecision = false
	ctx.RoshanAtEnd.KnowledgeState = "known_dead_from_kill"
	for i := range ctx.EnemyLaneStructuresAtEnd {
		state := &ctx.EnemyLaneStructuresAtEnd[i]
		state.Tier1KnownAlive = false
		state.Tier2KnownAlive = false
		state.Tier3KnownAlive = false
		if state.Lane == "mid" {
			state.Tier1Destroyed = true
			state.Tier2Destroyed = true
			state.Tier3KnownAlive = true
		}
	}

	withoutSupport := assessObjectiveMiss(tl, ctx)
	if withoutSupport.Candidate {
		t.Fatalf("unsupported T3 emitted candidate: %#v", withoutSupport)
	}

	tl.CreepClusters.Frames = []timeline.CreepClusterFrame{{
		T: 118,
		Clusters: []timeline.CreepCluster{{
			Team: 2, CenterX: 170, CenterY: 170, CreepCount: 5, LaneCreepCount: 4, SiegeCreepCount: 1,
			MaxMemberDistanceWorld: 250,
		}},
	}}
	withSupport := assessObjectiveMiss(tl, ctx)
	if !withSupport.Candidate {
		t.Fatalf("supported T3 suppressed: %#v", withSupport)
	}
	if len(withSupport.Evidence.EnemyPushableTowerOptions) != 1 || withSupport.Evidence.EnemyPushableTowerOptions[0] != (ObjectiveTowerOption{Lane: "mid", Tier: 3}) {
		t.Fatalf("T3 pushable evidence = %#v", withSupport.Evidence.EnemyPushableTowerOptions)
	}
}

func TestObjectiveCreepBackdoorSupportFailsClosedOnClusterSpread(t *testing.T) {
	clusters := timeline.CreepClusterTimeline{
		Available: true,
		Frames: []timeline.CreepClusterFrame{{
			T: 100,
			Clusters: []timeline.CreepCluster{{
				Team: 2, CenterX: 100, CenterY: 100, CreepCount: 2, LaneCreepCount: 2,
				MaxMemberDistanceWorld: 901,
			}},
		}},
	}
	if objectiveCreepBackdoorSupport(clusters, 2, 100, 100, 900, 90, 110) {
		t.Fatal("cluster whose member spread exceeds mechanic radius should fail closed")
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
		CreepClusters: timeline.CreepClusterTimeline{Available: true, Frames: []timeline.CreepClusterFrame{}},
		LaneStructures: timeline.LaneStructureTimeline{
			Available: true,
			InitialTowers: []timeline.LaneStructureInitialTower{
				{Team: 3, Lane: "top", Tier: 1, X: 180, Y: 180},
				{Team: 3, Lane: "top", Tier: 2, X: 175, Y: 175},
				{Team: 3, Lane: "top", Tier: 3, X: 170, Y: 170},
				{Team: 3, Lane: "mid", Tier: 1, X: 155, Y: 155},
				{Team: 3, Lane: "mid", Tier: 2, X: 150, Y: 150},
				{Team: 3, Lane: "mid", Tier: 3, X: 165, Y: 165},
				{Team: 3, Lane: "bottom", Tier: 1, X: 180, Y: 140},
				{Team: 3, Lane: "bottom", Tier: 2, X: 175, Y: 150},
				{Team: 3, Lane: "bottom", Tier: 3, X: 170, Y: 160},
			},
			InitialForts: []timeline.LaneStructureInitialFort{{Team: 3, X: 171, Y: 167}},
		},
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
