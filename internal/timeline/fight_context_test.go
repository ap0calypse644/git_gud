package timeline

import (
	"math"
	"testing"
)

func TestDeriveTargetFightContextsCapturesParticipationEvidence(t *testing.T) {
	targetSlot := 1
	allySlot := 0
	enemySlot := 128
	tl := &MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*PlayerTimeline{
			slotKey(targetSlot): {
				PlayerSlot: targetSlot,
				Team:       2,
				Samples: []HeroSample{
					{T: 9.5, X: 103, Y: 104, HP: 900, MaxHP: 1000, Mana: 400, MaxMana: 500, Level: 10, Alive: true},
					{T: 11, X: 102, Y: 103, Alive: true},
					{T: 12, X: 101, Y: 102, Alive: true},
					{T: 19, X: 100, Y: 101, Alive: false},
				},
			},
			slotKey(allySlot): {
				PlayerSlot: allySlot,
				Team:       2,
				Samples: []HeroSample{
					{T: 9.5, X: 104, Y: 104, Alive: true},
					{T: 10.5, X: 104, Y: 104, Alive: false},
				},
			},
			slotKey(2): {
				PlayerSlot: 2,
				Team:       2,
				Samples:    []HeroSample{{T: 9.5, X: 130, Y: 130, Alive: false}},
			},
			slotKey(enemySlot): {
				PlayerSlot: enemySlot,
				Team:       3,
				Samples:    []HeroSample{{T: 12, X: 100, Y: 100, Alive: true}},
			},
		},
		Fights: []FightWindow{{
			StartT: 7, EndT: 25,
			ObservedStartT: 10, ObservedEndT: 20,
			CenterX: 100, CenterY: 100,
			Participants: []int{0, 1, 128}, Deaths: 2, HeroDamage: 2500, TargetInvolved: true,
		}},
		Damage: []DamageEvent{
			{T: 12, AttackerSlot: targetSlot, VictimSlot: enemySlot, Value: 300},
			{T: 15, AttackerSlot: enemySlot, VictimSlot: targetSlot, Value: 200},
		},
		Abilities: []AbilityEvent{{T: 11, PlayerSlot: targetSlot, Ability: "test_spell"}},
		Deaths: []DeathEvent{
			{T: 10.5, VictimSlot: intPtrFight(allySlot), AttackerSlot: intPtrFight(enemySlot)},
			{T: 13, VictimSlot: intPtrFight(2), AttackerSlot: intPtrFight(enemySlot)},
			{T: 19, VictimSlot: intPtrFight(targetSlot), AttackerSlot: intPtrFight(enemySlot)},
		},
	}

	contexts := DeriveTargetFightContexts(tl)
	if len(contexts) != 1 {
		t.Fatalf("got %d contexts, want 1", len(contexts))
	}
	ctx := contexts[0]
	if !ctx.ObservedTimingAvailable || !ctx.TargetInvolved {
		t.Fatalf("timing=%v target_involved=%v", ctx.ObservedTimingAvailable, ctx.TargetInvolved)
	}
	if !ctx.TargetAtStart.SampleAvailable || !ctx.TargetAtStart.Alive {
		t.Fatalf("unexpected target start state: %+v", ctx.TargetAtStart)
	}
	if math.Abs(ctx.TargetAtStart.DistanceToFightCenter-5) > 1e-9 {
		t.Fatalf("distance to center=%v, want 5", ctx.TargetAtStart.DistanceToFightCenter)
	}
	if len(ctx.TeammatesAtStart) != 2 {
		t.Fatalf("teammates=%d, want 2", len(ctx.TeammatesAtStart))
	}
	if ctx.TeammatesAtStart[1].PlayerSlot != 2 || ctx.TeammatesAtStart[1].Alive {
		t.Fatalf("expected slot 2 dead at start, got %+v", ctx.TeammatesAtStart[1])
	}
	if len(ctx.NearbyAlliesAtStart) != 1 || ctx.NearbyAlliesAtStart[0].PlayerSlot != allySlot {
		t.Fatalf("nearby allies=%+v", ctx.NearbyAlliesAtStart)
	}
	if ctx.TargetFirstInvolvementT == nil || *ctx.TargetFirstInvolvementT != 11 || ctx.TargetFirstInvolvementSource != "ability" {
		t.Fatalf("first involvement=%v source=%q", ctx.TargetFirstInvolvementT, ctx.TargetFirstInvolvementSource)
	}
	if ctx.SecondsToFirstInvolvement == nil || *ctx.SecondsToFirstInvolvement != 1 {
		t.Fatalf("delay=%v, want 1", ctx.SecondsToFirstInvolvement)
	}
	if ctx.TargetDamageDealt != 300 || ctx.TargetDamageReceived != 200 || ctx.TargetAbilityCount != 1 {
		t.Fatalf("contribution dealt=%d received=%d abilities=%d", ctx.TargetDamageDealt, ctx.TargetDamageReceived, ctx.TargetAbilityCount)
	}
	if ctx.TargetDeathT == nil || *ctx.TargetDeathT != 19 {
		t.Fatalf("death=%v, want 19", ctx.TargetDeathT)
	}
	if len(ctx.AlliedDeathsBeforeTargetInvolvement) != 1 || ctx.AlliedDeathsBeforeTargetInvolvement[0] != allySlot {
		t.Fatalf("allied deaths before involvement=%v, want [0]", ctx.AlliedDeathsBeforeTargetInvolvement)
	}
}

func TestDeriveTargetFightContextsIncludesFightTargetDidNotJoin(t *testing.T) {
	tl := &MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			slotKey(1):   {PlayerSlot: 1, Team: 2, Samples: []HeroSample{{T: 29.5, X: 80, Y: 80, Alive: true}}},
			slotKey(0):   {PlayerSlot: 0, Team: 2, Samples: []HeroSample{{T: 29.5, X: 150, Y: 150, Alive: true}}},
			slotKey(128): {PlayerSlot: 128, Team: 3},
		},
		Fights: []FightWindow{{
			StartT: 27, EndT: 45,
			ObservedStartT: 30, ObservedEndT: 40,
			CenterX: 150, CenterY: 150,
			Participants: []int{0, 128}, Deaths: 1, HeroDamage: 2000,
		}},
	}

	contexts := DeriveTargetFightContexts(tl)
	if len(contexts) != 1 {
		t.Fatalf("got %d contexts, want 1", len(contexts))
	}
	ctx := contexts[0]
	if ctx.TargetInvolved {
		t.Fatal("target should not be marked involved")
	}
	if ctx.TargetFirstInvolvementT != nil || ctx.TargetDamageDealt != 0 || ctx.TargetDamageReceived != 0 {
		t.Fatalf("unexpected target contribution: %+v", ctx)
	}
	if !ctx.TargetAtStart.SampleAvailable || ctx.TargetAtStart.DistanceToFightCenter <= 90 {
		t.Fatalf("expected target start position far from fight, got %+v", ctx.TargetAtStart)
	}
}

func TestOverlappingFightContributionAssignedOnceByTargetPosition(t *testing.T) {
	tl := &MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			slotKey(1): {
				PlayerSlot: 1,
				Team:       2,
				Samples: []HeroSample{
					{T: 10, X: 100, Y: 100, Alive: true},
					{T: 12, X: 101, Y: 100, Alive: true},
				},
			},
			slotKey(128): {PlayerSlot: 128, Team: 3},
		},
		Fights: []FightWindow{
			{StartT: 7, EndT: 25, ObservedStartT: 10, ObservedEndT: 20, CenterX: 100, CenterY: 100, Participants: []int{1, 128}, TargetInvolved: true},
			{StartT: 7, EndT: 25, ObservedStartT: 10, ObservedEndT: 20, CenterX: 160, CenterY: 160, Participants: []int{1, 129}, TargetInvolved: true},
		},
		Damage: []DamageEvent{{T: 12, AttackerSlot: 1, VictimSlot: 128, Value: 500}},
	}

	contexts := DeriveTargetFightContexts(tl)
	if len(contexts) != 2 {
		t.Fatalf("got %d contexts, want 2", len(contexts))
	}
	if contexts[0].TargetDamageDealt != 500 || contexts[0].TargetFirstInvolvementT == nil {
		t.Fatalf("near fight did not receive contribution: %+v", contexts[0])
	}
	if contexts[1].TargetDamageDealt != 0 || contexts[1].TargetFirstInvolvementT != nil {
		t.Fatalf("overlapping far fight duplicated contribution: %+v", contexts[1])
	}
}

func TestFightContextLeavesStaleStartSampleUnavailable(t *testing.T) {
	tl := &MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*PlayerTimeline{
			slotKey(1): {PlayerSlot: 1, Team: 2, Samples: []HeroSample{{T: 5, X: 100, Y: 100, Alive: true}}},
		},
		Fights: []FightWindow{{StartT: 17, EndT: 30, ObservedStartT: 20, ObservedEndT: 25, CenterX: 100, CenterY: 100}},
	}

	contexts := DeriveTargetFightContexts(tl)
	if len(contexts) != 1 {
		t.Fatalf("got %d contexts, want 1", len(contexts))
	}
	if contexts[0].TargetAtStart.SampleAvailable {
		t.Fatalf("stale sample should be unavailable: %+v", contexts[0].TargetAtStart)
	}
}

func intPtrFight(v int) *int {
	return &v
}
