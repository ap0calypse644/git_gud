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
	if got.Team != dotaTeamRadiant || got.Method != "observer_ward_and_allied_hero_conservative_radius" {
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

func TestConservativeWardVisionRangeUsesSmallerPositiveRadius(t *testing.T) {
	ward := WardInterval{DayVisionRange: 1800, NightVisionRange: 800}
	if got := conservativeWardVisionRange(ward); got != 800 {
		t.Fatalf("got %.0f, want conservative 800", got)
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

func TestAlliedHeroVisionUsesFriendlyAlivePrimarySources(t *testing.T) {
	enemy := &PlayerTimeline{
		PlayerSlot: 128,
		Team:       dotaTeamDire,
		Samples: []HeroSample{
			{T: 10, X: 105, Y: 100, Alive: true},
			{T: 11, X: 105, Y: 100, Alive: true},
		},
	}
	sources := []HeroVisionSample{
		{T: 10.1, PlayerSlot: 0, Team: dotaTeamRadiant, X: 100, Y: 100, Alive: true, DayVisionRange: 1800, NightVisionRange: 800},
		{T: 11.1, PlayerSlot: 0, Team: dotaTeamRadiant, X: 100, Y: 100, Alive: true, DayVisionRange: 1800, NightVisionRange: 800},
		{T: 10.1, PlayerSlot: 129, Team: dotaTeamDire, X: 105, Y: 100, Alive: true, DayVisionRange: 1800, NightVisionRange: 800},
		{T: 10.1, PlayerSlot: 2, Team: dotaTeamRadiant, X: 105, Y: 100, Alive: false, DayVisionRange: 1800, NightVisionRange: 800},
	}

	idx := indexHeroVisionSources(dotaTeamRadiant, sources)
	got := deriveEstimatedVisibilityForPlayer(enemy, dotaTeamRadiant, nil, idx)
	if len(got) != 1 {
		t.Fatalf("got %d intervals, want 1", len(got))
	}
	if len(got[0].SourceHeroSlots) != 1 || got[0].SourceHeroSlots[0] != 0 {
		t.Fatalf("unexpected hero sources: %#v", got[0].SourceHeroSlots)
	}
}

func TestAlliedHeroVisionUsesConservativeNightRange(t *testing.T) {
	source := HeroVisionSample{
		T: 10, PlayerSlot: 0, Team: dotaTeamRadiant, X: 100, Y: 100, Alive: true,
		DayVisionRange: 1800, NightVisionRange: 800,
	}
	idx := indexHeroVisionSources(dotaTeamRadiant, []HeroVisionSample{source})

	inside := alliedHeroesCoveringSample(HeroSample{T: 10, X: 106.2, Y: 100, Alive: true}, dotaTeamRadiant, idx)
	outside := alliedHeroesCoveringSample(HeroSample{T: 10, X: 106.3, Y: 100, Alive: true}, dotaTeamRadiant, idx)
	if len(inside) != 1 {
		t.Fatalf("6.2 timeline units should be inside conservative 800-world-unit radius")
	}
	if len(outside) != 0 {
		t.Fatalf("6.3 timeline units should be outside conservative 800-world-unit radius")
	}
}

func TestEnemyKnowledgeAtVisibleLastSeenAndNeverSeen(t *testing.T) {
	k := KnowledgeTimeline{EstimatedVisibility: []EstimatedVisibilityInterval{
		{
			PlayerSlot: 128,
			StartT: 10,
			EndT:   20,
			EndX:   110,
			EndY:   120,
			SourceWards: []VisionSourceRef{{EntityIndex: 7, EntitySerial: 3}},
			SourceHeroSlots: []int{1, 2},
		},
	}}

	visible := EnemyKnowledgeAt(k, 128, 15)
	if visible.Status != "estimated_visible" || visible.SecondsSinceSeen == nil || *visible.SecondsSinceSeen != 0 {
		t.Fatalf("unexpected visible state: %#v", visible)
	}
	if visible.LastSeenT == nil || *visible.LastSeenT != 15 {
		t.Fatalf("visible last-seen time = %#v, want query time 15", visible.LastSeenT)
	}
	if visible.LastSeenX != nil || visible.LastSeenY != nil || len(visible.SourceWards) != 0 || len(visible.SourceHeroSlots) != 0 {
		t.Fatalf("current interval leaked future position/source evidence: %#v", visible)
	}

	missing := EnemyKnowledgeAt(k, 128, 35)
	if missing.Status != "last_seen" || missing.SecondsSinceSeen == nil || *missing.SecondsSinceSeen != 15 {
		t.Fatalf("unexpected last-seen state: %#v", missing)
	}
	if missing.LastSeenX == nil || *missing.LastSeenX != 110 || missing.LastSeenY == nil || *missing.LastSeenY != 120 {
		t.Fatalf("unexpected last-seen position: %#v", missing)
	}
	if len(missing.SourceWards) != 1 || len(missing.SourceHeroSlots) != 2 {
		t.Fatalf("last-seen evidence was not retained: %#v", missing)
	}

	never := EnemyKnowledgeAt(k, 129, 35)
	if never.Status != "never_seen" || never.LastSeenT != nil || never.SecondsSinceSeen != nil {
		t.Fatalf("unexpected never-seen state: %#v", never)
	}
}

func TestEnemyKnowledgeAtDoesNotUseFutureIntervals(t *testing.T) {
	k := KnowledgeTimeline{EstimatedVisibility: []EstimatedVisibilityInterval{
		{PlayerSlot: 128, StartT: 40, EndT: 50, EndX: 200, EndY: 200},
	}}
	got := EnemyKnowledgeAt(k, 128, 30)
	if got.Status != "never_seen" {
		t.Fatalf("future visibility leaked into state: %#v", got)
	}
}
