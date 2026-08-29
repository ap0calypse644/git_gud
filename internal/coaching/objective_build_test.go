package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputIncludesCompactObjectiveReviewInterval(t *testing.T) {
	tl := objectiveCoachingFixtureTimeline()
	got := BuildMatchCoachingInput(tl)

	var moment *CoachingMoment
	for i := range got.Moments {
		if got.Moments[i].Type == detector.TypePostFightConversionReviewCandidate {
			moment = &got.Moments[i]
			break
		}
	}
	if moment == nil {
		t.Fatalf("objective review missing from coaching moments: %#v", got.Moments)
	}
	if moment.StartT != 100 || moment.EndT != 160 {
		t.Fatalf("objective review span=%v..%v, want fight start through window end 100..160", moment.StartT, moment.EndT)
	}

	evidence, ok := moment.Evidence.(PostFightConversionReviewEvidence)
	if !ok {
		t.Fatalf("objective evidence type=%T", moment.Evidence)
	}
	if evidence.FightIndex != 12 || evidence.WindowStartT != 125 || evidence.WindowEndT != 160 || evidence.WindowDurationSeconds != 35 {
		t.Fatalf("objective timing evidence=%#v", evidence)
	}
	if evidence.AlliedDeaths != 0 || evidence.EnemyDeaths != 1 || evidence.EnemyDeathAdvantage != 1 || evidence.EnemyDeathsStillDeadAtEnd != 1 {
		t.Fatalf("objective fight outcome evidence=%#v", evidence)
	}
	if len(evidence.PushableTowerOptions) != 1 || evidence.PushableTowerOptions[0] != (ObjectiveReviewTowerOption{Lane: "bottom", Tier: 1}) {
		t.Fatalf("pushable tower evidence=%#v", evidence.PushableTowerOptions)
	}
	if evidence.RoshanKnownAliveForDecision || evidence.RoshanKnowledgeState != "known_dead_from_kill" {
		t.Fatalf("Roshan knowledge evidence=%#v", evidence)
	}
	if !evidence.NoTargetTeamConversion || evidence.TargetTeamConversionCount != 0 {
		t.Fatalf("conversion evidence=%#v", evidence)
	}
}

func TestObjectiveReviewCoachingJSONExcludesRawAndReplayOnlyEvidence(t *testing.T) {
	got := BuildMatchCoachingInput(objectiveCoachingFixtureTimeline())
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal coaching input: %v", err)
	}
	text := string(encoded)

	for _, required := range []string{
		`"type":"post_fight_conversion_review_candidate"`,
		`"pushable_tower_options":[{"lane":"bottom","tier":1}]`,
		`"roshan_knowledge_state":"known_dead_from_kill"`,
		`"enemy_deaths_still_dead_at_window_end":1`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("coaching input missing %q: %s", required, text)
		}
	}

	for _, forbidden := range []string{
		`"enemy_death_victim_slots"`,
		`"enemy_front_tower_options"`,
		`"enemy_tier1s_destroyed_at_end"`,
		`"enemy_lane_structures_at_end"`,
		`"creep_clusters"`,
		`"initial_towers"`,
		`"initial_forts"`,
		`"world_state"`,
		`"last_spawn_t"`,
		`"last_kill_t"`,
		`"x"`,
		`"y"`,
		"777.125",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("objective coaching input leaked %q: %s", forbidden, text)
		}
	}
}

func TestObjectiveCandidateEvidenceFailsClosedOnMalformedOrUnknownShape(t *testing.T) {
	if _, _, _, ok := objectiveCandidateEvidence(detector.ObjectiveCandidate{Type: "unknown"}); ok {
		t.Fatal("unknown objective candidate type crossed coaching boundary")
	}

	malformed := detector.ObjectiveCandidate{
		Type: detector.TypePostFightConversionReviewCandidate,
		Objective: &detector.ObjectiveMissEvidence{
			FightObservedStartT: 100,
			FightObservedEndT:   120,
			WindowStartT:        110,
			WindowEndT:          160,
		},
	}
	if _, _, _, ok := objectiveCandidateEvidence(malformed); ok {
		t.Fatal("malformed objective timing crossed coaching boundary")
	}
}

func objectiveCoachingFixtureTimeline() *timeline.MatchTimeline {
	enemySlot := 128
	return &timeline.MatchTimeline{
		MatchID:          4242,
		TargetPlayerSlot: 1,
		Players: map[string]*timeline.PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Team:       2,
				HeroName:   "npc_dota_hero_slark",
				Samples: []timeline.HeroSample{
					{T: 120, X: 40, Y: 40, Alive: true},
					{T: 159, X: 41, Y: 41, Alive: true},
				},
			},
			"128": {
				PlayerSlot: 128,
				Team:       3,
				HeroName:   "npc_dota_hero_axe",
				Samples: []timeline.HeroSample{
					{T: 114, X: 777.125, Y: 888.25, Alive: true},
					{T: 159, X: 777.125, Y: 888.25, Alive: false},
				},
			},
		},
		Deaths: []timeline.DeathEvent{{T: 115, VictimSlot: &enemySlot}},
		CreepClusters: timeline.CreepClusterTimeline{
			Available: true,
			Frames: []timeline.CreepClusterFrame{{
				T: 130,
				Clusters: []timeline.CreepCluster{{
					Team: 2, CenterX: 150, CenterY: 150, CreepCount: 4, LaneCreepCount: 4,
				}},
			}},
		},
		LaneStructures: timeline.LaneStructureTimeline{
			Available: true,
			InitialTowers: []timeline.LaneStructureInitialTower{
				{Team: 3, Lane: "bottom", Tier: 1, X: 180, Y: 140},
				{Team: 3, Lane: "mid", Tier: 1, X: 155, Y: 155},
			},
			InitialForts: []timeline.LaneStructureInitialFort{{Team: 3, X: 171, Y: 167}},
		},
		TargetPostFightObjectives: timeline.PostFightObjectiveTimeline{
			Available: true,
			Contexts: []timeline.PostFightObjectiveContext{{
				FightIndex:                12,
				ObservedTimingAvailable:   true,
				FightObservedStartT:       100,
				FightObservedEndT:         120,
				WindowStartT:              125,
				WindowEndT:                160,
				WindowEndReason:           "next_fight_start",
				WindowDurationSeconds:     35,
				TargetTeam:                2,
				TargetInvolved:            true,
				Participants:              []int{1, 128},
				FightDeaths:               1,
				FightHeroDamage:           2400,
				AlliedDeaths:              0,
				EnemyDeaths:               1,
				EnemyDeathAdvantage:       1,
				TargetEndSampleAvailable:  true,
				TargetEndSampleT:          120,
				TargetAliveAtEnd:          true,
				AlliedEndSamplesAvailable: 5,
				AlliedHeroesAliveAtEnd:    5,
				EnemyLaneStructuresAtEnd: []timeline.LaneStructureState{
					{
						Team: 3, Lane: "mid", T: 120,
						Tier1PresentAtStart: true,
						Tier1Destroyed:      true,
					},
					{
						Team: 3, Lane: "bottom", T: 120,
						Tier1PresentAtStart: true,
						Tier1KnownAlive:     true,
					},
				},
				RoshanAtEnd: timeline.RoshanPostFightState{
					ReplayStateAvailable:  true,
					WorldState:            "dead",
					KnowledgeState:        "known_dead_from_kill",
					KnownAliveForDecision: false,
				},
				TargetTeamConversions: []timeline.PostFightObjectiveEvent{},
			}},
		},
	}
}
