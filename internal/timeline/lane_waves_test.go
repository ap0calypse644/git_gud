package timeline

import "testing"

func TestAssignCreepActivationPrefersWaitingTransition(t *testing.T) {
	previous := creepState{waiting: true}
	current := creepState{waiting: false}

	got, source := assignCreepActivation(previous, true, current, false, 30.4)
	if source != "waiting_transition" {
		t.Fatalf("source = %q, want waiting_transition", source)
	}
	if !got.cohortKnown || got.cohortSecond != 30 {
		t.Fatalf("cohort = (%v,%d), want (true,30)", got.cohortKnown, got.cohortSecond)
	}

	carried, source := assignCreepActivation(got, true, current, false, 31.2)
	if source != "" {
		t.Fatalf("carried source = %q, want empty", source)
	}
	if !carried.cohortKnown || carried.cohortSecond != 30 {
		t.Fatalf("carried cohort = (%v,%d), want (true,30)", carried.cohortKnown, carried.cohortSecond)
	}
}

func TestAssignCreepActivationFallbacksAreExplicit(t *testing.T) {
	created, source := assignCreepActivation(creepState{}, false, creepState{}, true, 60.6)
	if source != "created_active" || created.cohortSecond != 61 {
		t.Fatalf("created = source %q cohort %d, want created_active/61", source, created.cohortSecond)
	}

	observed, source := assignCreepActivation(creepState{}, false, creepState{}, false, 89.6)
	if source != "first_observed_active" || observed.cohortSecond != 90 {
		t.Fatalf("observed = source %q cohort %d, want first_observed_active/90", source, observed.cohortSecond)
	}
}

func TestCohortComponentsDoNotMergeConsecutiveWaves(t *testing.T) {
	states := map[creepEntityKey]creepState{
		{index: 1, serial: 1}: {
			key: creepEntityKey{index: 1, serial: 1}, team: 2, kind: "lane",
			x: 100, y: 100, alive: true, cohortKnown: true, cohortSecond: 30,
		},
		{index: 2, serial: 1}: {
			key: creepEntityKey{index: 2, serial: 1}, team: 2, kind: "lane",
			x: 101, y: 101, alive: true, cohortKnown: true, cohortSecond: 60,
		},
	}

	components := cohortCreepComponents(states, creepClusterRadiusTimeline)
	if len(components) != 2 {
		t.Fatalf("component count = %d, want 2", len(components))
	}
	if components[0].cohortSecond == components[1].cohortSecond {
		t.Fatalf("cohorts unexpectedly equal: %d", components[0].cohortSecond)
	}
}

func TestDeriveLaneWavesLabelsThreeLanesConservatively(t *testing.T) {
	frames := make([]rawWaveFrame, 0, 10)
	for second := 1; second <= 10; second++ {
		top := rawWaveComponent{
			team: 2, cohortSecond: 0,
			centerX: 80, centerY: 90 + float64(second)*1.2,
			creepCount: 5, laneCreepCount: 5,
		}
		bottom := rawWaveComponent{
			team: 2, cohortSecond: 0,
			centerX: 90 + float64(second)*1.2, centerY: 80,
			creepCount: 5, laneCreepCount: 5,
		}
		mid := rawWaveComponent{
			team: 2, cohortSecond: 0,
			centerX: 115 + float64(second)*0.5, centerY: 115 + float64(second)*0.5,
			creepCount: 5, laneCreepCount: 5,
		}
		frames = append(frames, rawWaveFrame{T: float64(second), Components: []rawWaveComponent{top, mid, bottom}})
	}

	got := deriveLaneWaveTimeline(frames, LaneWaveActivationEvidence{WaitingTransitions: 15}, true)
	if !got.Available {
		t.Fatal("timeline unavailable")
	}
	if got.UnknownTrackCount != 0 {
		t.Fatalf("unknown tracks = %d, want 0", got.UnknownTrackCount)
	}
	if len(got.Waves) != 3 {
		t.Fatalf("waves = %d, want 3", len(got.Waves))
	}
	want := []string{"top", "mid", "bottom"}
	for i, lane := range want {
		if got.Waves[i].Lane != lane {
			t.Fatalf("wave[%d].lane = %q, want %q", i, got.Waves[i].Lane, lane)
		}
		if len(got.Waves[i].Samples) != 10 {
			t.Fatalf("wave[%d] samples = %d, want 10", i, len(got.Waves[i].Samples))
		}
	}
}

func TestDeriveLaneWavesLeavesAmbiguousCohortUnknown(t *testing.T) {
	frames := make([]rawWaveFrame, 0, 8)
	for second := 1; second <= 8; second++ {
		frames = append(frames, rawWaveFrame{
			T: float64(second),
			Components: []rawWaveComponent{{
				team: 3, cohortSecond: 30,
				centerX: 120 + float64(second)*0.25,
				centerY: 120 + float64(second)*0.25,
				creepCount: 5, laneCreepCount: 5,
			}},
		})
	}

	got := deriveLaneWaveTimeline(frames, LaneWaveActivationEvidence{}, true)
	if got.UnknownTrackCount != 1 {
		t.Fatalf("unknown tracks = %d, want 1", got.UnknownTrackCount)
	}
	if len(got.Waves) != 0 {
		t.Fatalf("waves = %d, want 0", len(got.Waves))
	}
	if got.Available {
		t.Fatal("ambiguous-only timeline should not claim available lane waves")
	}
}
