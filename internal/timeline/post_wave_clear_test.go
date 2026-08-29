package timeline

import "testing"

func TestDeriveTargetPostWaveOverstayPreservesPostClearDelta(t *testing.T) {
	tl := &MatchTimeline{
		TargetWaveDanger: TargetWaveDangerTimeline{
			Available: true,
			Contexts: []TargetWaveDangerContext{{
				WaveID:         "3:60:bottom",
				Lane:           "bottom",
				EnemyTeam:      3,
				SpawnT:         60,
				LastDepletionT: 99,
				EndT:           100,
				ExposureEndT:   104,
				Snapshots: []TargetWaveDangerSnapshot{
					{
						Kind:                    "last_depletion",
						T:                       99,
						TargetAvailable:         true,
						TargetAlive:             true,
						LaneProgressAvailable:   true,
						TargetLaneProgressWorld: 4900,
					},
					{
						Kind:                    "end",
						T:                       100,
						TargetAvailable:         true,
						TargetAlive:             true,
						LaneProgressAvailable:   true,
						TargetLaneProgressWorld: 5000,
					},
					{
						Kind:                    "exposure_end",
						T:                       104,
						TargetAvailable:         true,
						TargetAlive:             true,
						LaneProgressAvailable:   true,
						TargetLaneProgressWorld: 5300,
					},
				},
			}},
		},
	}

	got := DeriveTargetPostWaveOverstay(tl)
	if len(got.Contexts) != 1 {
		t.Fatalf("contexts = %d, want 1", len(got.Contexts))
	}
	ctx := got.Contexts[0]
	if ctx.LastDepletionState.T != 99 {
		t.Fatalf("last depletion state t = %v, want 99", ctx.LastDepletionState.T)
	}
	if ctx.PostClear.DurationSeconds != 5 {
		t.Fatalf("post-clear duration = %v, want 5", ctx.PostClear.DurationSeconds)
	}
	if ctx.PostClear.LaneProgressDeltaWorld == nil || *ctx.PostClear.LaneProgressDeltaWorld != 400 {
		t.Fatalf("post-clear lane delta = %#v, want +400", ctx.PostClear.LaneProgressDeltaWorld)
	}
	if ctx.PostPrimary.DurationSeconds != 4 {
		t.Fatalf("post-primary duration = %v, want 4", ctx.PostPrimary.DurationSeconds)
	}
	if ctx.PostPrimary.LaneProgressDeltaWorld == nil || *ctx.PostPrimary.LaneProgressDeltaWorld != 300 {
		t.Fatalf("post-primary lane delta = %#v, want +300", ctx.PostPrimary.LaneProgressDeltaWorld)
	}
}
