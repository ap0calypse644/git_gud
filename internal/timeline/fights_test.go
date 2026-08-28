package timeline

import "testing"

func TestDeriveFightWindows(t *testing.T) {
	target := 1
	other := 128
	timeline := &MatchTimeline{
		TargetPlayerSlot: target,
		DurationSeconds:  120,
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

func intPtrForTest(v int) *int { return &v }
