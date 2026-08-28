package timeline

import (
	"math"
	"testing"
)

func TestDeriveTargetWaveTakingEmitsDepletingEnemyWavePeriod(t *testing.T) {
	tl := MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Team:       2,
				Samples: []HeroSample{
					{T: 1, X: 100, Y: 100, Alive: true},
					{T: 2, X: 100, Y: 100, Alive: true},
					{T: 3, X: 100, Y: 100, Alive: true},
				},
			},
		},
		LaneWaves: LaneWaveTimeline{
			Available: true,
			Waves: []LaneWave{
				{
					ID:     "3:0:mid",
					Team:   3,
					SpawnT: 0,
					Lane:   "mid",
					Samples: []LaneWaveSample{
						{T: 1, CenterX: 101, CenterY: 100, CreepCount: 4},
						{T: 2, CenterX: 101, CenterY: 100, CreepCount: 3},
						{T: 3, CenterX: 101, CenterY: 100, CreepCount: 2},
					},
				},
				{
					ID:     "2:0:mid",
					Team:   2,
					SpawnT: 0,
					Lane:   "mid",
					Samples: []LaneWaveSample{
						{T: 1, CenterX: 100, CenterY: 100, CreepCount: 4},
						{T: 2, CenterX: 100, CenterY: 100, CreepCount: 3},
						{T: 3, CenterX: 100, CenterY: 100, CreepCount: 2},
					},
				},
			},
		},
	}

	got := DeriveTargetWaveTaking(&tl)
	if !got.Available {
		t.Fatal("expected target-wave capability to be available")
	}
	if got.ExposurePeriodsObserved != 1 {
		t.Fatalf("exposure periods = %d, want 1", got.ExposurePeriodsObserved)
	}
	if len(got.Periods) != 1 {
		t.Fatalf("periods = %d, want 1", len(got.Periods))
	}
	p := got.Periods[0]
	if p.WaveID != "3:0:mid" || p.EnemyTeam != 3 || p.Lane != "mid" {
		t.Fatalf("unexpected wave identity: %+v", p)
	}
	if p.StartT != 1 || p.EndT != 3 || p.ContactSamples != 3 {
		t.Fatalf("unexpected period bounds: %+v", p)
	}
	if p.ObservedCreepLoss != 2 || p.NetCreepCountChange != -2 {
		t.Fatalf("unexpected depletion: %+v", p)
	}
	if math.Abs(p.MinDistanceWorld-128) > 0.001 || math.Abs(p.MeanDistanceWorld-128) > 0.001 {
		t.Fatalf("unexpected distance summary: %+v", p)
	}
}

func TestDeriveTargetWaveTakingRejectsNoDepletion(t *testing.T) {
	tl := targetWaveTakingTestTimeline([]int{4, 4, 4})
	got := DeriveTargetWaveTaking(&tl)
	if got.ExposurePeriodsObserved != 1 || got.RejectedNoDepletion != 1 {
		t.Fatalf("unexpected rejection counts: %+v", got)
	}
	if len(got.Periods) != 0 {
		t.Fatalf("periods = %d, want 0", len(got.Periods))
	}
}

func TestDeriveTargetWaveTakingRejectsShortContact(t *testing.T) {
	tl := targetWaveTakingTestTimeline([]int{4, 3})
	got := DeriveTargetWaveTaking(&tl)
	if got.ExposurePeriodsObserved != 1 || got.RejectedTooShort != 1 {
		t.Fatalf("unexpected rejection counts: %+v", got)
	}
	if len(got.Periods) != 0 {
		t.Fatalf("periods = %d, want 0", len(got.Periods))
	}
}

func TestFreshHeroSampleAtOrBeforeIsCausalAndFresh(t *testing.T) {
	player := &PlayerTimeline{Samples: []HeroSample{
		{T: 1.0, X: 200, Y: 200, Alive: true},
		{T: 2.1, X: 100, Y: 100, Alive: true},
	}}

	sample, ok := freshHeroSampleAtOrBefore(player, 2.0, 1.5)
	if !ok {
		t.Fatal("expected causal sample")
	}
	if sample.T != 1.0 || sample.X != 200 {
		t.Fatalf("future sample leaked into lookup: %+v", sample)
	}
	if _, ok := freshHeroSampleAtOrBefore(player, 3.7, 1.5); ok {
		t.Fatal("expected stale sample to be rejected")
	}
}

func TestDeriveTargetWaveTakingUnavailableFailsClosed(t *testing.T) {
	got := DeriveTargetWaveTaking(&MatchTimeline{})
	if got.Available {
		t.Fatal("unexpected available capability")
	}
	if got.Periods == nil {
		t.Fatal("periods must be an explicit empty slice")
	}
	if len(got.Periods) != 0 {
		t.Fatalf("periods = %d, want 0", len(got.Periods))
	}
}

func targetWaveTakingTestTimeline(counts []int) MatchTimeline {
	targetSamples := make([]HeroSample, 0, len(counts))
	waveSamples := make([]LaneWaveSample, 0, len(counts))
	for i, count := range counts {
		ts := float64(i + 1)
		targetSamples = append(targetSamples, HeroSample{T: ts, X: 100, Y: 100, Alive: true})
		waveSamples = append(waveSamples, LaneWaveSample{T: ts, CenterX: 101, CenterY: 100, CreepCount: count})
	}
	return MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			"1": {PlayerSlot: 1, Team: 2, Samples: targetSamples},
		},
		LaneWaves: LaneWaveTimeline{
			Available: true,
			Waves: []LaneWave{{
				ID:      "3:0:mid",
				Team:    3,
				SpawnT:  0,
				Lane:    "mid",
				Samples: waveSamples,
			}},
		},
	}
}
