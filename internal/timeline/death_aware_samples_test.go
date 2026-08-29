package timeline

import "testing"

func TestFreshLivingHeroSampleAtOrBeforeRejectsInterveningDeathAndAllowsLaterRespawnSample(t *testing.T) {
	targetSlot := 1
	tl := MatchTimeline{
		Deaths: []DeathEvent{{T: 3.5, VictimSlot: &targetSlot}},
	}
	player := &PlayerTimeline{
		PlayerSlot: targetSlot,
		Samples: []HeroSample{
			{T: 3, X: 100, Y: 100, Alive: true},
			{T: 5, X: 120, Y: 120, Alive: true},
		},
	}

	if _, ok := freshLivingHeroSampleAtOrBefore(&tl, player, 4, 1.5); ok {
		t.Fatal("pre-death alive sample must not remain live after exact death")
	}

	sample, ok := freshLivingHeroSampleAtOrBefore(&tl, player, 5, 1.5)
	if !ok || sample.T != 5 {
		t.Fatalf("later alive sample should naturally restore live state: sample=%+v ok=%v", sample, ok)
	}
}

func TestDeriveTargetWaveTakingStopsExposureAtExactDeath(t *testing.T) {
	targetSlot := 1
	tl := MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*PlayerTimeline{
			"1": {
				PlayerSlot: targetSlot,
				Team:       2,
				Samples: []HeroSample{
					{T: 1, X: 100, Y: 100, Alive: true},
					{T: 2, X: 100, Y: 100, Alive: true},
					{T: 3, X: 100, Y: 100, Alive: true},
				},
			},
		},
		Deaths: []DeathEvent{{T: 3.5, VictimSlot: &targetSlot}},
		LaneWaves: LaneWaveTimeline{
			Available: true,
			Waves: []LaneWave{{
				ID: "3:0:bottom", Team: 3, SpawnT: 0, Lane: "bottom",
				Samples: []LaneWaveSample{
					{T: 1, CenterX: 101, CenterY: 100, CreepCount: 4},
					{T: 2, CenterX: 101, CenterY: 100, CreepCount: 3},
					{T: 3, CenterX: 101, CenterY: 100, CreepCount: 3},
					{T: 4, CenterX: 101, CenterY: 100, CreepCount: 2},
				},
			}},
		},
	}

	got := DeriveTargetWaveTaking(&tl)
	if len(got.Periods) != 1 {
		t.Fatalf("periods = %d, want 1: %+v", len(got.Periods), got.Periods)
	}
	period := got.Periods[0]
	if period.ExposureEndT != 3 {
		t.Fatalf("post-death t=4 contact leaked into exposure: %+v", period)
	}
	if period.LastDepletionT != 2 || period.ObservedCreepLoss != 1 {
		t.Fatalf("post-death creep loss leaked into wave-taking evidence: %+v", period)
	}
}

func TestWaveTakingNearbyAlliesExcludesInterveningExactDeath(t *testing.T) {
	allySlot := 2
	tl := MatchTimeline{
		Players: map[string]*PlayerTimeline{
			"2": {
				PlayerSlot: allySlot,
				Team:       2,
				Samples: []HeroSample{{T: 9, X: 101, Y: 100, Alive: true}},
			},
		},
		Deaths: []DeathEvent{{T: 9.5, VictimSlot: &allySlot}},
	}

	allies := waveTakingNearbyAlliesAt(&tl, 2, 1, 10, 100, 100)
	if len(allies) != 0 {
		t.Fatalf("dead ally must not remain support via stale alive sample: %+v", allies)
	}
}
