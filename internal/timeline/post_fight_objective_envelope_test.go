package timeline

import "testing"

func TestPostFightEnvelopeUsesConsolidatedFightBounds(t *testing.T) {
	tl := MatchTimeline{
		DurationSeconds: 100,
		Fights: []FightWindow{
			{StartT: 7, EndT: 25, ObservedStartT: 10, ObservedEndT: 20},
			{StartT: 28, EndT: 50, ObservedStartT: 31, ObservedEndT: 45},
		},
	}

	_, currentEnd := postFightEnvelopeBounds(tl.Fights[0])
	if currentEnd != 25 {
		t.Fatalf("current envelope end = %v, want 25", currentEnd)
	}
	end, reason := nextPostFightBoundary(&tl, 0, currentEnd)
	if end != 28 || reason != "next_fight_start" {
		t.Fatalf("boundary = (%v, %q), want (28, next_fight_start)", end, reason)
	}
}

func TestPostFightEnvelopeClosesWhenAnotherConsolidatedFightOverlaps(t *testing.T) {
	tl := MatchTimeline{
		DurationSeconds: 100,
		Fights: []FightWindow{
			{StartT: 7, EndT: 25, ObservedStartT: 10, ObservedEndT: 20},
			{StartT: 22, EndT: 40, ObservedStartT: 27, ObservedEndT: 35},
		},
	}

	_, currentEnd := postFightEnvelopeBounds(tl.Fights[0])
	end, reason := nextPostFightBoundary(&tl, 0, currentEnd)
	if end != 25 || reason != "overlapping_fight_active" {
		t.Fatalf("boundary = (%v, %q), want (25, overlapping_fight_active)", end, reason)
	}
}

func TestPostFightEnvelopeFallsBackToObservedBounds(t *testing.T) {
	fight := FightWindow{ObservedStartT: 10, ObservedEndT: 20}
	start, end := postFightEnvelopeBounds(fight)
	if start != 10 || end != 20 {
		t.Fatalf("fallback envelope = (%v, %v), want (10, 20)", start, end)
	}
}
