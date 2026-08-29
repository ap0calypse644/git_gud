package timeline

import "testing"

func TestDeriveTargetWaveDangerContextCausalEvidence(t *testing.T) {
	pastTower := 9.0
	futureTower := 11.0
	tl := &MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Team:       2,
				Samples: []HeroSample{
					{T: 9, X: 100, Y: 100, Alive: true},
					{T: 11, X: 150, Y: 150, Alive: true},
				},
			},
			"2": {
				PlayerSlot: 2,
				Team:       2,
				Samples: []HeroSample{
					{T: 9, X: 101, Y: 100, Alive: true},
					{T: 11, X: 180, Y: 180, Alive: true},
				},
			},
			"5": {PlayerSlot: 5, Team: 3},
		},
		LaneWaves: LaneWaveTimeline{
			Available: true,
			Waves: []LaneWave{{
				ID: "3:60:bottom", Team: 3, SpawnT: 60, Lane: "bottom",
				Samples: []LaneWaveSample{
					{T: 9, CenterX: 102, CenterY: 100, CreepCount: 4},
					{T: 11, CenterX: 160, CenterY: 160, CreepCount: 2},
				},
			}},
		},
		TargetWaveTaking: TargetWaveTakingTimeline{
			Available: true,
			Periods: []TargetWaveTakingPeriod{{
				WaveID: "3:60:bottom", Lane: "bottom", EnemyTeam: 3, SpawnT: 60,
				StartT: 10, EndT: 10, ExposureStartT: 10, ExposureEndT: 10,
				FirstDepletionT: 10, LastDepletionT: 10, ObservedCreepLoss: 2,
			}},
		},
		LaneStructures: LaneStructureTimeline{
			Available: true,
			Events: []LaneStructureEvent{
				{T: pastTower, Team: 2, Lane: "bottom", Tier: 1},
				{T: futureTower, Team: 3, Lane: "bottom", Tier: 1},
			},
		},
		Knowledge: KnowledgeTimeline{
			Team: 2,
			EstimatedVisibility: []EstimatedVisibilityInterval{{
				PlayerSlot: 5, StartT: 8, EndT: 12,
				StartX: 80, StartY: 80, EndX: 170, EndY: 170,
			}},
		},
	}

	got := DeriveTargetWaveDangerContext(tl)
	if !got.Available {
		t.Fatal("expected dangerous-wave context capability to be available")
	}
	if got.LaneProgressAvailable {
		t.Fatal("lane progress must remain unavailable until geometry is validated")
	}
	if len(got.Contexts) != 1 || len(got.Contexts[0].Snapshots) != 1 {
		t.Fatalf("expected one context with one deduplicated snapshot, got %#v", got.Contexts)
	}

	s := got.Contexts[0].Snapshots[0]
	if !s.TargetAvailable || s.TargetSampleT != 9 || s.TargetX != 100 || s.TargetY != 100 {
		t.Fatalf("target lookup used future or wrong sample: %#v", s)
	}
	if !s.WaveAvailable || s.WaveSampleT != 9 || s.WaveX != 102 || s.CreepCount != 4 {
		t.Fatalf("wave lookup used future or wrong sample: %#v", s)
	}
	if len(s.NearbyAllies) != 1 || s.NearbyAllies[0].SampleT != 9 || s.NearbyAllies[0].DistanceWorld != 128 {
		t.Fatalf("ally context not causal: %#v", s.NearbyAllies)
	}
	if !s.FriendlyStructuresAvailable || !s.FriendlyStructures.Tier1Destroyed || s.FriendlyStructures.Tier1DestroyedAt == nil || *s.FriendlyStructures.Tier1DestroyedAt != pastTower {
		t.Fatalf("expected past friendly tower destruction: %#v", s.FriendlyStructures)
	}
	if !s.EnemyStructuresAvailable || s.EnemyStructures.Tier1Destroyed || s.EnemyStructures.Tier1DestroyedAt != nil {
		t.Fatalf("future enemy tower destruction leaked into earlier snapshot: %#v", s.EnemyStructures)
	}
	if len(s.EnemyKnowledge) != 1 || s.EnemyKnowledge[0].Status != "estimated_visible" {
		t.Fatalf("unexpected enemy knowledge: %#v", s.EnemyKnowledge)
	}
	if s.EnemyKnowledge[0].LastSeenX != nil || s.EnemyKnowledge[0].LastSeenY != nil {
		t.Fatalf("active visibility leaked future endpoint position: %#v", s.EnemyKnowledge[0])
	}
}

func TestDeriveTargetWaveDangerContextUnavailableFailsClosed(t *testing.T) {
	got := DeriveTargetWaveDangerContext(&MatchTimeline{})
	if got.Available {
		t.Fatal("expected unavailable without target wave-taking capability")
	}
	if got.Contexts == nil || len(got.Contexts) != 0 {
		t.Fatalf("contexts must be an empty list, got %#v", got.Contexts)
	}
}
