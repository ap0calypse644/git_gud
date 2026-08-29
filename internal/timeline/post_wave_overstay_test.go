package timeline

import "testing"

func TestDeriveTargetPostWaveOverstayEvidence(t *testing.T) {
	targetSlot := 1
	endAge := 5.0
	exposureAgeA := 9.0
	exposureAgeB := 10.0

	tl := &MatchTimeline{
		TargetPlayerSlot: targetSlot,
		TargetWaveDanger: TargetWaveDangerTimeline{
			Available: true,
			Contexts: []TargetWaveDangerContext{{
				WaveID:            "3:60:bottom",
				Lane:              "bottom",
				EnemyTeam:         3,
				SpawnT:            60,
				FirstDepletionT:   97,
				LastDepletionT:    99,
				EndT:              100,
				ExposureEndT:      104,
				ObservedCreepLoss: 3,
				Snapshots: []TargetWaveDangerSnapshot{
					{
						Kind:                                      "end",
						T:                                         100,
						TargetAvailable:                           true,
						TargetAlive:                               true,
						LaneProgressAvailable:                     true,
						TargetLaneProgressWorld:                   5000,
						TargetLaneOffsetWorld:                     100,
						FriendlyRetreatReferenceAvailable:         true,
						FriendlyRetreatReferenceTier:              2,
						TargetForwardOfFriendlyReferenceWorld:     2000,
						EnemyForwardReferenceAvailable:            true,
						EnemyForwardReferenceTier:                 1,
						TargetForwardOfEnemyReferenceWorld:        -2500,
						NearbyAllies: []WaveTakingNearbyAlly{{DistanceWorld: 1000}},
						EnemyKnowledge: []EnemyKnowledgeState{
							{PlayerSlot: 128, Status: "estimated_visible"},
							{PlayerSlot: 129, Status: "last_seen", SecondsSinceSeen: &endAge},
							{PlayerSlot: 130, Status: "never_seen"},
						},
					},
					{
						Kind:                                      "exposure_end",
						T:                                         104,
						TargetAvailable:                           true,
						TargetAlive:                               true,
						LaneProgressAvailable:                     true,
						TargetLaneProgressWorld:                   5300,
						TargetLaneOffsetWorld:                     120,
						FriendlyRetreatReferenceAvailable:         true,
						FriendlyRetreatReferenceTier:              2,
						TargetForwardOfFriendlyReferenceWorld:     2300,
						EnemyForwardReferenceAvailable:            true,
						EnemyForwardReferenceTier:                 1,
						TargetForwardOfEnemyReferenceWorld:        -2200,
						NearbyAllies: []WaveTakingNearbyAlly{{DistanceWorld: 1600}},
						EnemyKnowledge: []EnemyKnowledgeState{
							{PlayerSlot: 128, Status: "last_seen", SecondsSinceSeen: &exposureAgeA},
							{PlayerSlot: 129, Status: "last_seen", SecondsSinceSeen: &exposureAgeB},
							{PlayerSlot: 130, Status: "never_seen"},
						},
					},
				},
			}},
		LaneWaves: LaneWaveTimeline{
			Available: true,
			Waves: []LaneWave{
				{ID: "3:70:mid", Team: 3, Lane: "mid", SpawnT: 70},
				{ID: "2:75:bottom", Team: 2, Lane: "bottom", SpawnT: 75},
				{ID: "3:90:bottom", Team: 3, Lane: "bottom", SpawnT: 90},
				{ID: "3:120:bottom", Team: 3, Lane: "bottom", SpawnT: 120},
			},
		},
		TargetWaveTaking: TargetWaveTakingTimeline{
			Available: true,
			Periods: []TargetWaveTakingPeriod{{
				WaveID:         "3:90:bottom",
				Lane:           "bottom",
				EnemyTeam:      3,
				SpawnT:         90,
				StartT:         110,
				EndT:           112,
				ExposureStartT: 109,
				ExposureEndT:   114,
			}},
		},
		Deaths: []DeathEvent{
			{T: 95, VictimSlot: &targetSlot},
			{T: 115, VictimSlot: &targetSlot},
		},
	}

	got := DeriveTargetPostWaveOverstay(tl)
	if !got.Available || got.Method != targetPostWaveOverstayMethod {
		t.Fatalf("unexpected capability: %#v", got)
	}
	if len(got.Contexts) != 1 {
		t.Fatalf("contexts = %d, want 1", len(got.Contexts))
	}
	ctx := got.Contexts[0]
	if ctx.PrimaryEndT != 100 || ctx.ExposureEndT != 104 || ctx.PostPrimary.DurationSeconds != 4 {
		t.Fatalf("unexpected post-primary window: %#v", ctx)
	}
	if ctx.PostPrimary.LaneProgressDeltaWorld == nil || *ctx.PostPrimary.LaneProgressDeltaWorld != 300 {
		t.Fatalf("lane progress delta = %#v, want +300", ctx.PostPrimary.LaneProgressDeltaWorld)
	}
	if ctx.PostPrimary.NearestAllyDistanceDeltaWorld == nil || *ctx.PostPrimary.NearestAllyDistanceDeltaWorld != 600 {
		t.Fatalf("nearest ally delta = %#v, want +600", ctx.PostPrimary.NearestAllyDistanceDeltaWorld)
	}
	if ctx.PostPrimary.EstimatedVisibleEnemiesDelta != -1 || ctx.PostPrimary.MissingEnemiesDelta != 1 {
		t.Fatalf("unexpected enemy-knowledge deltas: %#v", ctx.PostPrimary)
	}
	if ctx.PostPrimary.MaxLastSeenAgeDeltaSeconds == nil || *ctx.PostPrimary.MaxLastSeenAgeDeltaSeconds != 5 {
		t.Fatalf("last-seen age delta = %#v, want +5", ctx.PostPrimary.MaxLastSeenAgeDeltaSeconds)
	}

	if !ctx.NextCohort.Available || ctx.NextCohort.WaveID != "3:90:bottom" {
		t.Fatalf("wrong next same-lane enemy cohort: %#v", ctx.NextCohort)
	}
	if !ctx.NextCohort.TargetTakingObserved || ctx.NextCohort.TakingStartT == nil || *ctx.NextCohort.TakingStartT != 110 {
		t.Fatalf("expected later target wave-taking evidence: %#v", ctx.NextCohort)
	}
	if ctx.NextCohort.SecondsFromPrimaryEndToTaking == nil || *ctx.NextCohort.SecondsFromPrimaryEndToTaking != 10 {
		t.Fatalf("seconds to next taking = %#v, want 10", ctx.NextCohort.SecondsFromPrimaryEndToTaking)
	}
	if ctx.NextCohort.TakingOverlapsPrimaryEnd {
		t.Fatalf("later period should not overlap primary end: %#v", ctx.NextCohort)
	}

	if ctx.Outcome.NextTargetDeathT == nil || *ctx.Outcome.NextTargetDeathT != 115 {
		t.Fatalf("next target death = %#v, want 115", ctx.Outcome.NextTargetDeathT)
	}
	if ctx.Outcome.SecondsFromPrimaryEndToDeath == nil || *ctx.Outcome.SecondsFromPrimaryEndToDeath != 15 {
		t.Fatalf("seconds to death = %#v, want 15", ctx.Outcome.SecondsFromPrimaryEndToDeath)
	}
}

func TestDeriveTargetPostWaveOverstayFailsClosedWithoutM16(t *testing.T) {
	got := DeriveTargetPostWaveOverstay(&MatchTimeline{})
	if got.Available {
		t.Fatal("M17 evidence must fail closed without M16 causal context")
	}
	if got.Contexts == nil || len(got.Contexts) != 0 {
		t.Fatalf("contexts must be an empty list, got %#v", got.Contexts)
	}
}

func TestNextTakingPeriodForWaveAllowsOverlapAtPrimaryEnd(t *testing.T) {
	periods := []TargetWaveTakingPeriod{{
		WaveID: "3:90:bottom", StartT: 98, EndT: 102, ExposureStartT: 97, ExposureEndT: 105,
	}}
	period, ok := nextTakingPeriodForWave(periods, "3:90:bottom", 100)
	if !ok || period.StartT != 98 {
		t.Fatalf("expected overlapping next-wave period, got period=%#v ok=%v", period, ok)
	}
}
