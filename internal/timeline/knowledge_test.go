package timeline

import "testing"

func TestDeriveKnowledgeUsesFriendlyObserverWardRadius(t *testing.T) {
	tl := MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Team:       dotaTeamRadiant,
				Samples:    []HeroSample{{T: 10, X: 50, Y: 50, Alive: true}},
			},
			"128": {
				PlayerSlot: 128,
				Team:       dotaTeamDire,
				Samples: []HeroSample{
					{T: 10, X: 100, Y: 100, Alive: true},
					{T: 11, X: 110, Y: 100, Alive: true},
					{T: 12, X: 114, Y: 100, Alive: true},
				},
			},
		},
		VisionSources: VisionSourceTimeline{Wards: []WardInterval{
			{
				EntityIndex:      7,
				EntitySerial:     3,
				Kind:             "observer",
				Team:             dotaTeamRadiant,
				X:                100,
				Y:                100,
				StartT:           0,
				EndT:             100,
				DayVisionRange:   1600,
				NightVisionRange: 1600,
			},
		}},
	}

	got := DeriveKnowledge(&tl)
	if got.Team != dotaTeamRadiant || got.Method != "observer_ward_radius_only" {
		t.Fatalf("unexpected knowledge header: %#v", got)
	}
	if len(got.EstimatedVisibility) != 1 {
		t.Fatalf("got %d intervals, want 1", len(got.EstimatedVisibility))
	}
	iv := got.EstimatedVisibility[0]
	if iv.PlayerSlot != 128 || iv.StartT != 10 || iv.EndT != 11 || iv.SampleCount != 2 {
		t.Fatalf("unexpected interval: %#v", iv)
	}
	if len(iv.SourceWards) != 1 || iv.SourceWards[0].EntityIndex != 7 || iv.SourceWards[0].EntitySerial != 3 {
		t.Fatalf("unexpected source refs: %#v", iv.SourceWards)
	}
}

func TestDeriveKnowledgeIgnoresEnemyObserversAndSentries(t *testing.T) {
	player := &PlayerTimeline{
		PlayerSlot: 128,
		Team:       dotaTeamDire,
		Samples:    []HeroSample{{T: 10, X: 100, Y: 100, Alive: true}},
	}
	wards := []WardInterval{
		{
			EntityIndex: 1, Kind: "observer", Team: dotaTeamDire,
			X: 100, Y: 100, StartT: 0, EndT: 20, DayVisionRange: 1600, NightVisionRange: 1600,
		},
		{
			EntityIndex: 2, Kind: "sentry", Team: dotaTeamRadiant,
			X: 100, Y: 100, StartT: 0, EndT: 20, DayVisionRange: 1600, NightVisionRange: 1600,
		},
	}

	got := deriveEstimatedWardVisibilityForPlayer(player, dotaTeamRadiant, wards)
	if len(got) != 0 {
		t.Fatalf("got estimated visibility from invalid sources: %#v", got)
	}
}

func TestWardVisionRangeWorldToTimelineScale(t *testing.T) {
	ward := WardInterval{
		EntityIndex: 1,
		Kind:             "observer",
		Team:             dotaTeamRadiant,
		X:                100,
		Y:                100,
		StartT:           0,
		EndT:             20,
		DayVisionRange:   1600,
		NightVisionRange: 1600,
	}

	inside := observerWardsCoveringSample(HeroSample{T: 10, X: 112.4, Y: 100, Alive: true}, dotaTeamRadiant, []WardInterval{ward})
	outside := observerWardsCoveringSample(HeroSample{T: 10, X: 112.6, Y: 100, Alive: true}, dotaTeamRadiant, []WardInterval{ward})
	if len(inside) != 1 {
		t.Fatalf("12.4 timeline units should be inside 1600-world-unit radius")
	}
	if len(outside) != 0 {
		t.Fatalf("12.6 timeline units should be outside 1600-world-unit radius")
	}
}

func TestDeriveKnowledgeSplitsLongSampleGaps(t *testing.T) {
	player := &PlayerTimeline{
		PlayerSlot: 128,
		Team:       dotaTeamDire,
		Samples: []HeroSample{
			{T: 10, X: 100, Y: 100, Alive: true},
			{T: 14, X: 100, Y: 100, Alive: true},
		},
	}
	ward := WardInterval{
		EntityIndex: 1, Kind: "observer", Team: dotaTeamRadiant,
		X: 100, Y: 100, StartT: 0, EndT: 20, DayVisionRange: 1600, NightVisionRange: 1600,
	}

	got := deriveEstimatedWardVisibilityForPlayer(player, dotaTeamRadiant, []WardInterval{ward})
	if len(got) != 2 {
		t.Fatalf("got %d intervals, want 2 for a long observation gap", len(got))
	}
}
