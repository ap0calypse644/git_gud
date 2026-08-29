package detector

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAnalyzePostWavesEmitsPostClearCombatCandidate(t *testing.T) {
	progress := 300.0
	depth := 2500.0
	first := 101.25
	secondsToCombat := 1.25
	death := 108.0
	secondsToDeath := 7.0

	tl := &timeline.MatchTimeline{
		MatchID: 123,
		TargetPostWaveOverstay: timeline.TargetPostWaveOverstayTimeline{
			Available: true,
			Contexts: []timeline.TargetPostWaveOverstayContext{{
				WaveID:       "3:60:bottom",
				Lane:         "bottom",
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

	got := AnalyzePostWaves(tl)
	if got.MatchID != 123 || len(got.Assessments) != 1 || len(got.Candidates) != 1 {
		t.Fatalf("unexpected analysis: %#v", got)
	}
	candidate := got.Candidates[0]
	if candidate.Type != TypePostWaveOverstayCandidate || candidate.Confidence != ConfidenceLow || candidate.T != 100 {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
	if candidate.PostWave == nil {
		t.Fatal("candidate missing post-wave evidence")
	}
	if candidate.PostWave.PostClearLaneProgressDeltaWorld == nil || *candidate.PostWave.PostClearLaneProgressDeltaWorld != 300 {
		t.Fatalf("forward delta = %#v, want 300", candidate.PostWave.PostClearLaneProgressDeltaWorld)
	}
	if candidate.PostWave.SecondsFromClearToFirstInvolvement == nil || *candidate.PostWave.SecondsFromClearToFirstInvolvement != 1.25 {
		t.Fatalf("seconds to combat = %#v, want 1.25", candidate.PostWave.SecondsFromClearToFirstInvolvement)
	}
	if candidate.PostWave.NextTargetDeathT == nil || *candidate.PostWave.NextTargetDeathT != 108 {
		t.Fatalf("retrospective death evidence = %#v, want 108", candidate.PostWave.NextTargetDeathT)
	}
}

func TestAssessPostWaveOverstayRejectsBoundaryControls(t *testing.T) {
	positive := 100.0
	negative := -100.0
	first := 103.0
	seconds := 3.0

	base := timeline.TargetPostWaveOverstayContext{
		WaveID:         "3:60:bottom",
		Lane:           "bottom",
		LastDepletionT: 100,
		PrimaryEndT:    101,
		ExposureEndT:   104,
		LastDepletionState: timeline.TargetPostWaveState{
			TargetAvailable: true,
			TargetAlive:     true,
		},
		PostClear: timeline.TargetPostWaveChange{
			DurationSeconds:        4,
			LaneProgressDeltaWorld: &positive,
		},
		CombatContext: timeline.TargetPostWaveCombatContext{
			TargetCombatStartedDuringPostClear:         true,
			TargetFirstInvolvementT:                    &first,
			SecondsFromLastDepletionToFirstInvolvement: &seconds,
		},
	}

	tests := []struct {
		name   string
		mutate func(*timeline.TargetPostWaveOverstayContext)
	}{
		{
			name: "retreating",
			mutate: func(ctx *timeline.TargetPostWaveOverstayContext) {
				ctx.PostClear.LaneProgressDeltaWorld = &negative
			},
		},
		{
			name: "combat already started at clear",
			mutate: func(ctx *timeline.TargetPostWaveOverstayContext) {
				ctx.CombatContext.TargetCombatStartedByLastDepletion = true
			},
		},
		{
			name: "no combat during tail",
			mutate: func(ctx *timeline.TargetPostWaveOverstayContext) {
				ctx.CombatContext.TargetCombatStartedDuringPostClear = false
			},
		},
		{
			name: "dead at clear",
			mutate: func(ctx *timeline.TargetPostWaveOverstayContext) {
				ctx.LastDepletionState.TargetAlive = false
			},
		},
		{
			name: "zero exposure tail",
			mutate: func(ctx *timeline.TargetPostWaveOverstayContext) {
				ctx.PostClear.DurationSeconds = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := base
			tt.mutate(&ctx)
			if got := assessPostWaveOverstay(ctx); got.Candidate {
				t.Fatalf("unexpected candidate: %#v", got)
			}
		})
	}
}

func TestAnalyzePostWavesFailsClosedWithoutM17(t *testing.T) {
	got := AnalyzePostWaves(&timeline.MatchTimeline{MatchID: 456})
	if got.MatchID != 456 {
		t.Fatalf("match id = %d, want 456", got.MatchID)
	}
	if got.Assessments == nil || got.Candidates == nil {
		t.Fatalf("analysis slices must be non-nil: %#v", got)
	}
	if len(got.Assessments) != 0 || len(got.Candidates) != 0 {
		t.Fatalf("expected empty analysis without M17 evidence: %#v", got)
	}
}
