package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputIncludesCompactPostWaveOverstayInterval(t *testing.T) {
	got := BuildMatchCoachingInput(postWaveCoachingFixtureTimeline())

	var moment *CoachingMoment
	for i := range got.Moments {
		if got.Moments[i].Type == detector.TypePostWaveOverstayCandidate {
			moment = &got.Moments[i]
			break
		}
	}
	if moment == nil {
		t.Fatalf("post-wave overstay missing from coaching moments: %#v", got.Moments)
	}
	if moment.StartT != 100 || moment.EndT != 104 {
		t.Fatalf("post-wave review span=%v..%v, want clear through exposure end 100..104", moment.StartT, moment.EndT)
	}
	if moment.Confidence != detector.ConfidenceLow {
		t.Fatalf("confidence=%q, want low", moment.Confidence)
	}

	evidence, ok := moment.Evidence.(PostWaveOverstayReviewEvidence)
	if !ok {
		t.Fatalf("post-wave evidence type=%T", moment.Evidence)
	}
	if evidence.WaveID != "3:60:bottom" || evidence.Lane != "bottom" {
		t.Fatalf("wave evidence=%#v", evidence)
	}
	if evidence.LastDepletionT != 100 || evidence.ExposureEndT != 104 || evidence.PostClearDurationSeconds != 4 {
		t.Fatalf("timing evidence=%#v", evidence)
	}
	if evidence.PostClearLaneProgressDeltaWorld != 300 || !evidence.TargetCombatStartedDuringPostClear || evidence.SecondsFromClearToFirstInvolvement != 1.25 {
		t.Fatalf("decision evidence=%#v", evidence)
	}
	if evidence.TargetFirstInvolvementSource != "damage_received" || evidence.NextWaveTakingObserved {
		t.Fatalf("context evidence=%#v", evidence)
	}
	if evidence.NextTargetDeathT == nil || *evidence.NextTargetDeathT != 108 {
		t.Fatalf("retrospective death evidence=%#v", evidence.NextTargetDeathT)
	}
}

func TestPostWaveOverstayCoachingJSONExcludesRawAndOmniscientEvidence(t *testing.T) {
	got := BuildMatchCoachingInput(postWaveCoachingFixtureTimeline())
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal coaching input: %v", err)
	}
	text := string(encoded)

	for _, required := range []string{
		`"type":"post_wave_overstay_candidate"`,
		`"wave_id":"3:60:bottom"`,
		`"lane":"bottom"`,
		`"post_clear_lane_progress_delta_world":300`,
		`"seconds_from_clear_to_first_involvement":1.25`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("coaching input missing %q: %s", required, text)
		}
	}

	for _, forbidden := range []string{
		`"depth_at_clear_world"`,
		`"fresh_living_allies_at_clear"`,
		`"missing_enemies_at_clear"`,
		`"seconds_from_primary_end_to_death"`,
		`"target_wave_danger"`,
		`"target_post_wave_overstay"`,
		`"knowledge"`,
		`"vision_sources"`,
		`"players"`,
		`"samples"`,
		`"x"`,
		`"y"`,
		"777.125",
		"888.25",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("post-wave coaching input leaked %q: %s", forbidden, text)
		}
	}
}

func TestPostWaveCandidateEvidenceFailsClosedOnMalformedOrUnknownShape(t *testing.T) {
	if _, _, _, ok := postWaveCandidateEvidence(detector.PostWaveCandidate{Type: "unknown"}); ok {
		t.Fatal("unknown post-wave candidate type crossed coaching boundary")
	}

	progress := 300.0
	seconds := 1.25
	first := 101.25
	malformed := detector.PostWaveCandidate{
		Type: detector.TypePostWaveOverstayCandidate,
		PostWave: &detector.PostWaveOverstayEvidence{
			WaveID:                              "3:60:bottom",
			Lane:                                "bottom",
			LastDepletionT:                      100,
			ExposureEndT:                        99,
			TargetAvailableAtClear:              true,
			TargetAliveAtClear:                  true,
			PostClearDurationSeconds:            4,
			PostClearLaneProgressDeltaWorld:     &progress,
			TargetCombatStartedDuringPostClear:  true,
			TargetFirstInvolvementT:             &first,
			SecondsFromClearToFirstInvolvement:  &seconds,
		},
	}
	if _, _, _, ok := postWaveCandidateEvidence(malformed); ok {
		t.Fatal("malformed post-wave timing crossed coaching boundary")
	}
}

func postWaveCoachingFixtureTimeline() *timeline.MatchTimeline {
	progress := 300.0
	depth := 2500.0
	first := 101.25
	secondsToCombat := 1.25
	death := 108.0
	secondsToDeath := 7.0

	return &timeline.MatchTimeline{
		MatchID:          4343,
		TargetPlayerSlot: 1,
		Players: map[string]*timeline.PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Team:       2,
				HeroName:   "npc_dota_hero_slark",
			},
			"128": {
				PlayerSlot: 128,
				Team:       3,
				HeroName:   "npc_dota_hero_axe",
				Samples: []timeline.HeroSample{{T: 100, X: 777.125, Y: 888.25, Alive: true}},
			},
		},
		TargetPostWaveOverstay: timeline.TargetPostWaveOverstayTimeline{
			Available: true,
			Contexts: []timeline.TargetPostWaveOverstayContext{{
				WaveID:         "3:60:bottom",
				Lane:           "bottom",
				LastDepletionT: 100,
				PrimaryEndT:    101,
				ExposureEndT:   104,
				LastDepletionState: timeline.TargetPostWaveState{
					TargetAvailable:                 true,
					TargetAlive:                     true,
					ForwardOfFriendlyReferenceWorld: &depth,
					FreshLivingAllies:               3,
					MissingEnemies:                  4,
				},
				PostClear: timeline.TargetPostWaveChange{
					DurationSeconds:        4,
					LaneProgressDeltaWorld: &progress,
				},
				CombatContext: timeline.TargetPostWaveCombatContext{
					TargetCombatStartedByLastDepletion:         false,
					TargetCombatStartedDuringPostClear:         true,
					TargetFirstInvolvementT:                    &first,
					SecondsFromLastDepletionToFirstInvolvement: &secondsToCombat,
					TargetFirstInvolvementSource:               "damage_received",
				},
				NextCohort: timeline.TargetPostWaveNextCohort{TargetTakingObserved: false},
				Outcome: timeline.TargetPostWaveOutcome{
					NextTargetDeathT:             &death,
					SecondsFromPrimaryEndToDeath: &secondsToDeath,
				},
			}},
		},
	}
}
