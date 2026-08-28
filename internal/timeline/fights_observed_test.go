package timeline

import "testing"

func TestFightWindowRetainsObservedCombatBoundaries(t *testing.T) {
	attacker := 128
	victim := 1
	tl := &MatchTimeline{
		TargetPlayerSlot: victim,
		DurationSeconds:  100,
		Players: map[string]*PlayerTimeline{
			"1": {
				PlayerSlot: victim,
				Samples:    []HeroSample{{T: 20, X: 100, Y: 100, Alive: false}},
			},
			"128": {
				PlayerSlot: attacker,
				Samples:    []HeroSample{{T: 20, X: 101, Y: 100, Alive: true}},
			},
		},
		Deaths: []DeathEvent{{T: 20, AttackerSlot: &attacker, VictimSlot: &victim}},
	}

	got := DeriveFightWindows(tl)
	if len(got) != 1 {
		t.Fatalf("got %d fights, want 1", len(got))
	}
	fight := got[0]
	if fight.ObservedStartT != 20 || fight.ObservedEndT != 20 {
		t.Fatalf("unexpected observed bounds: %#v", fight)
	}
	if fight.StartT != 17 || fight.EndT != 25 {
		t.Fatalf("unexpected padded bounds: %#v", fight)
	}
}

func TestMergeFightWindowsPreservesObservedCombatSpan(t *testing.T) {
	a := FightWindow{
		StartT: 10, EndT: 30,
		ObservedStartT: 13, ObservedEndT: 25,
		CenterX: 100, CenterY: 100,
		Participants: []int{1, 2, 128},
		HeroDamage: 1000,
	}
	b := FightWindow{
		StartT: 20, EndT: 40,
		ObservedStartT: 23, ObservedEndT: 35,
		CenterX: 101, CenterY: 100,
		Participants: []int{1, 2, 129},
		HeroDamage: 2000,
	}

	got := mergeFightWindows(a, b)
	if got.ObservedStartT != 13 || got.ObservedEndT != 35 {
		t.Fatalf("unexpected merged observed span: %#v", got)
	}
	if got.StartT != 10 || got.EndT != 40 {
		t.Fatalf("unexpected merged padded span: %#v", got)
	}
}
