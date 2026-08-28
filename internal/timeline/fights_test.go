package timeline

import "testing"

func TestDeriveFightWindows(t *testing.T) {
	target := 1
	other := 128
	timeline := &MatchTimeline{
		TargetPlayerSlot: target,
		DurationSeconds:  120,
		Players: map[string]*PlayerTimeline{
			"1":   {PlayerSlot: 1, Samples: samplesAt(100, 100, 0, 120)},
			"128": {PlayerSlot: 128, Samples: samplesAt(101, 100, 0, 120)},
			"2":   {PlayerSlot: 2, Samples: samplesAt(70, 160, 0, 120)},
			"129": {PlayerSlot: 129, Samples: samplesAt(71, 160, 0, 120)},
			"3":   {PlayerSlot: 3, Samples: samplesAt(150, 70, 0, 120)},
			"4":   {PlayerSlot: 4, Samples: samplesAt(151, 70, 0, 120)},
			"130": {PlayerSlot: 130, Samples: samplesAt(150, 71, 0, 120)},
		},
		Damage: []DamageEvent{
			{T: 10, AttackerSlot: target, VictimSlot: other, Value: 300},
			{T: 14, AttackerSlot: other, VictimSlot: target, Value: 350},
			{T: 40, AttackerSlot: 2, VictimSlot: 129, Value: 100},
		},
		Deaths: []DeathEvent{
			{T: 16, AttackerSlot: intPtrForTest(other), VictimSlot: intPtrForTest(target)},
			{T: 70, AttackerSlot: intPtrForTest(3), VictimSlot: intPtrForTest(130), AssistSlots: []int{4}},
		},
	}

	got := DeriveFightWindows(timeline)
	if len(got) != 2 {
		t.Fatalf("got %d fights, want 2: %#v", len(got), got)
	}
	if got[0].StartT != 7 || got[0].EndT != 21 {
		t.Fatalf("first window = %.1f..%.1f, want 7..21", got[0].StartT, got[0].EndT)
	}
	if got[0].HeroDamage != 650 || got[0].Deaths != 1 || !got[0].TargetInvolved {
		t.Fatalf("first fight = %#v", got[0])
	}
	if len(got[0].Participants) != 2 || got[0].Participants[0] != 1 || got[0].Participants[1] != 128 {
		t.Fatalf("participants = %#v", got[0].Participants)
	}
	if got[1].Deaths != 1 || got[1].TargetInvolved {
		t.Fatalf("second fight = %#v", got[1])
	}
}

func TestDeriveFightWindowsSeparatesSimultaneousMapAreas(t *testing.T) {
	timeline := &MatchTimeline{
		TargetPlayerSlot: 1,
		DurationSeconds:  60,
		Players: map[string]*PlayerTimeline{
			"0":   {PlayerSlot: 0, Samples: samplesAt(78, 160, 0, 60)},
			"1":   {PlayerSlot: 1, Samples: samplesAt(80, 160, 0, 60)},
			"128": {PlayerSlot: 128, Samples: samplesAt(81, 160, 0, 60)},
			"2":   {PlayerSlot: 2, Samples: samplesAt(126, 128, 0, 60)},
			"3":   {PlayerSlot: 3, Samples: samplesAt(128, 128, 0, 60)},
			"129": {PlayerSlot: 129, Samples: samplesAt(129, 128, 0, 60)},
		},
		Damage: []DamageEvent{
			{T: 10, AttackerSlot: 1, VictimSlot: 128, Value: 600},
			{T: 11, AttackerSlot: 128, VictimSlot: 1, Value: 650},
			{T: 10.5, AttackerSlot: 3, VictimSlot: 129, Value: 700},
			{T: 12, AttackerSlot: 2, VictimSlot: 129, Value: 750},
		},
		Deaths: []DeathEvent{
			{T: 13, AttackerSlot: intPtrForTest(0), VictimSlot: intPtrForTest(128), AssistSlots: []int{1}},
			{T: 13.5, AttackerSlot: intPtrForTest(3), VictimSlot: intPtrForTest(129), AssistSlots: []int{2}},
		},
	}

	got := DeriveFightWindows(timeline)
	if len(got) != 2 {
		t.Fatalf("got %d fights, want two spatially separate fights: %#v", len(got), got)
	}
	if got[0].CenterX > 100 || got[1].CenterX < 110 {
		t.Fatalf("fight centers were not spatially separated: %#v", got)
	}
}

func TestDeriveFightWindowsCapsContinuousLaneTrading(t *testing.T) {
	timeline := &MatchTimeline{
		TargetPlayerSlot: 1,
		DurationSeconds:  90,
		Players: map[string]*PlayerTimeline{
			"1":   {PlayerSlot: 1, Samples: samplesAt(90, 150, 0, 90)},
			"128": {PlayerSlot: 128, Samples: samplesAt(91, 150, 0, 90)},
		},
	}
	for sec := 0; sec <= 60; sec += 2 {
		timeline.Damage = append(timeline.Damage, DamageEvent{
			T: float64(sec), AttackerSlot: 1, VictimSlot: 128, Value: 200,
		})
	}

	got := DeriveFightWindows(timeline)
	if len(got) < 2 {
		t.Fatalf("continuous lane trading was chained into one fight: %#v", got)
	}
	for _, fight := range got {
		if duration := fight.EndT - fight.StartT; duration > fightMaxRawSpanSeconds+fightLeadSeconds+fightTrailSeconds {
			t.Fatalf("fight duration %.1fs exceeds cap: %#v", duration, fight)
		}
	}
}

func TestPlayerPositionAtUsesNearestSample(t *testing.T) {
	timeline := &MatchTimeline{Players: map[string]*PlayerTimeline{
		"1": {PlayerSlot: 1, Samples: []HeroSample{{T: 10, X: 90, Y: 100}, {T: 12, X: 92, Y: 102}}},
	}}
	x, y, ok := playerPositionAt(timeline, 1, 11.6)
	if !ok || x != 92 || y != 102 {
		t.Fatalf("playerPositionAt = %.1f,%.1f,%v; want 92,102,true", x, y, ok)
	}
	if _, _, ok := playerPositionAt(timeline, 1, 30); ok {
		t.Fatal("playerPositionAt accepted a stale sample")
	}
}

func samplesAt(x, y float64, start, end int) []HeroSample {
	out := make([]HeroSample, 0, end-start+1)
	for sec := start; sec <= end; sec++ {
		out = append(out, HeroSample{T: float64(sec), X: x, Y: y, Alive: true})
	}
	return out
}

func intPtrForTest(v int) *int { return &v }
