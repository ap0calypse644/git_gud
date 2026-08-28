package timeline

import "testing"

func TestConsolidateFightWindowsMergesConvergedClusters(t *testing.T) {
	in := []FightWindow{
		{
			StartT: 2277.7, EndT: 2320.4,
			CenterX: 149.1, CenterY: 75.7,
			Participants: []int{0, 1, 2, 3, 4, 128, 129, 130, 131, 132},
			Deaths: 2, HeroDamage: 14874, TargetInvolved: true,
		},
		{
			StartT: 2292.3, EndT: 2328.4,
			CenterX: 152.4, CenterY: 82.7,
			Participants: []int{0, 1, 2, 3, 4, 128, 129, 130, 131, 132},
			Deaths: 3, HeroDamage: 13567, TargetInvolved: true,
		},
	}

	got := ConsolidateFightWindows(in)
	if len(got) != 1 {
		t.Fatalf("got %d windows, want 1: %#v", len(got), got)
	}
	if got[0].StartT != 2277.7 || got[0].EndT != 2328.4 {
		t.Fatalf("merged window bounds = %.1f..%.1f", got[0].StartT, got[0].EndT)
	}
	if got[0].Deaths != 5 || got[0].HeroDamage != 28441 {
		t.Fatalf("merged totals = deaths %d damage %d", got[0].Deaths, got[0].HeroDamage)
	}
	if len(got[0].Participants) != 10 || !got[0].TargetInvolved {
		t.Fatalf("merged participants/target = %#v", got[0])
	}
}

func TestConsolidateFightWindowsKeepsSeparateMapAreas(t *testing.T) {
	in := []FightWindow{
		{
			StartT: 340, EndT: 390,
			CenterX: 176, CenterY: 91,
			Participants: []int{0, 1, 128, 132},
			Deaths: 2, HeroDamage: 3600,
		},
		{
			StartT: 355, EndT: 391,
			CenterX: 93, CenterY: 162,
			Participants: []int{1, 3, 128, 130},
			Deaths: 1, HeroDamage: 2400,
		},
	}

	got := ConsolidateFightWindows(in)
	if len(got) != 2 {
		t.Fatalf("spatially separate fights merged: %#v", got)
	}
}

func TestConsolidateFightWindowsRequiresSubstantialOverlap(t *testing.T) {
	in := []FightWindow{
		{
			StartT: 10, EndT: 20,
			CenterX: 100, CenterY: 100,
			Participants: []int{0, 1, 128, 129},
			Deaths: 1, HeroDamage: 3000,
		},
		{
			StartT: 18.5, EndT: 30,
			CenterX: 102, CenterY: 101,
			Participants: []int{0, 1, 128, 129},
			Deaths: 1, HeroDamage: 2500,
		},
	}

	got := ConsolidateFightWindows(in)
	if len(got) != 2 {
		t.Fatalf("briefly overlapping sequential fights merged: %#v", got)
	}
}

func TestConsolidateFightWindowsRequiresParticipantContinuity(t *testing.T) {
	in := []FightWindow{
		{
			StartT: 10, EndT: 30,
			CenterX: 100, CenterY: 100,
			Participants: []int{0, 1, 128, 129},
			Deaths: 1, HeroDamage: 3000,
		},
		{
			StartT: 15, EndT: 32,
			CenterX: 102, CenterY: 101,
			Participants: []int{2, 3, 130, 131},
			Deaths: 1, HeroDamage: 2500,
		},
	}

	got := ConsolidateFightWindows(in)
	if len(got) != 2 {
		t.Fatalf("participant-disjoint fights merged: %#v", got)
	}
}
