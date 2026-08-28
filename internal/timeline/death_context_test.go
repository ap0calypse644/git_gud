package timeline

import "testing"

func TestDeriveTargetDeathContextsBuildsDeterministicEvidence(t *testing.T) {
	targetSlot := 1
	killerSlot := 128
	tl := MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*PlayerTimeline{
			"0": {
				PlayerSlot: 0,
				Team:       dotaTeamRadiant,
				Samples:    []HeroSample{{T: 19.5, X: 103, Y: 104, Alive: true}},
			},
			"1": {
				PlayerSlot: targetSlot,
				Team:       dotaTeamRadiant,
				Samples:    []HeroSample{{T: 19.8, X: 100, Y: 100, Alive: false}},
			},
			"2": {
				PlayerSlot: 2,
				Team:       dotaTeamRadiant,
				Samples:    []HeroSample{{T: 20.1, X: 130, Y: 100, Alive: false}},
			},
			"128": {PlayerSlot: 128, Team: dotaTeamDire},
			"129": {PlayerSlot: 129, Team: dotaTeamDire},
		},
		Deaths: []DeathEvent{
			{T: 20, AttackerSlot: &killerSlot, VictimSlot: &targetSlot, AssistSlots: []int{129, 128}},
		},
		Damage: []DamageEvent{
			{T: 10, AttackerSlot: 128, VictimSlot: targetSlot, Value: 100},
			{T: 15, AttackerSlot: 128, VictimSlot: targetSlot, Value: 200},
			{T: 19, AttackerSlot: 129, VictimSlot: targetSlot, Value: 300},
			{T: 12, AttackerSlot: targetSlot, VictimSlot: 128, Value: 50},
			{T: 17, AttackerSlot: targetSlot, VictimSlot: 128, Value: 70},
			{T: 20.1, AttackerSlot: 128, VictimSlot: targetSlot, Value: 999},
		},
		Fights: []FightWindow{
			{
				StartT: 10, EndT: 25, CenterX: 101, CenterY: 101,
				Participants: []int{1, 128, 129}, Deaths: 2, HeroDamage: 2400, TargetInvolved: true,
			},
		},
		Knowledge: KnowledgeTimeline{EstimatedVisibility: []EstimatedVisibilityInterval{
			{PlayerSlot: 128, StartT: 18, EndT: 21, EndX: 110, EndY: 111},
			{PlayerSlot: 129, StartT: 5, EndT: 10, EndX: 90, EndY: 91},
		}},
	}

	got := DeriveTargetDeathContexts(&tl)
	if len(got) != 1 {
		t.Fatalf("got %d contexts, want 1", len(got))
	}
	ctx := got[0]
	if !ctx.PositionAvailable || ctx.X != 100 || ctx.Y != 100 {
		t.Fatalf("unexpected target position: %#v", ctx)
	}
	if ctx.KillerSlot == nil || *ctx.KillerSlot != 128 {
		t.Fatalf("unexpected killer: %#v", ctx.KillerSlot)
	}
	if len(ctx.AssistSlots) != 2 || ctx.AssistSlots[0] != 128 || ctx.AssistSlots[1] != 129 {
		t.Fatalf("assists not sorted/copied: %#v", ctx.AssistSlots)
	}
	if ctx.Fight == nil || ctx.Fight.StartT != 10 || ctx.Fight.EndT != 25 || ctx.Fight.TimeFromFightStart != 10 {
		t.Fatalf("unexpected fight context: %#v", ctx.Fight)
	}
	if len(ctx.NearbyAllies) != 1 || ctx.NearbyAllies[0].PlayerSlot != 0 || ctx.NearbyAllies[0].Distance != 5 {
		t.Fatalf("unexpected nearby allies: %#v", ctx.NearbyAllies)
	}
	if ctx.DamageReceivedLast5s != 500 || ctx.DamageReceivedLast10s != 600 {
		t.Fatalf("unexpected received damage windows: 5s=%d 10s=%d", ctx.DamageReceivedLast5s, ctx.DamageReceivedLast10s)
	}
	if ctx.DamageDealtLast5s != 70 || ctx.DamageDealtLast10s != 120 {
		t.Fatalf("unexpected dealt damage windows: 5s=%d 10s=%d", ctx.DamageDealtLast5s, ctx.DamageDealtLast10s)
	}
	if len(ctx.EnemyKnowledge) != 2 {
		t.Fatalf("got %d enemy states, want 2", len(ctx.EnemyKnowledge))
	}
	if ctx.EnemyKnowledge[0].PlayerSlot != 128 || ctx.EnemyKnowledge[0].Status != "estimated_visible" {
		t.Fatalf("unexpected first enemy state: %#v", ctx.EnemyKnowledge[0])
	}
	if ctx.EnemyKnowledge[1].PlayerSlot != 129 || ctx.EnemyKnowledge[1].Status != "last_seen" ||
		ctx.EnemyKnowledge[1].SecondsSinceSeen == nil || *ctx.EnemyKnowledge[1].SecondsSinceSeen != 10 {
		t.Fatalf("unexpected second enemy state: %#v", ctx.EnemyKnowledge[1])
	}
}

func TestTargetDeathContextFightBoundaryIsInclusive(t *testing.T) {
	for _, deathT := range []float64{10, 20} {
		t.Run("boundary", func(t *testing.T) {
			fight := FightWindow{StartT: 10, EndT: 20, Participants: []int{1, 128}, TargetInvolved: true}
			got := fightContextForDeath([]FightWindow{fight}, 1, deathT, 0, 0, false)
			if got == nil {
				t.Fatalf("death at %.1f should be inside inclusive fight window", deathT)
			}
		})
	}
}

func TestNearestHeroSampleAtRejectsStaleSample(t *testing.T) {
	player := &PlayerTimeline{Samples: []HeroSample{{T: 10, X: 1, Y: 2, Alive: true}}}
	if _, ok := nearestHeroSampleAt(player, 14); !ok {
		t.Fatalf("sample exactly at max age should be accepted")
	}
	if _, ok := nearestHeroSampleAt(player, 14.01); ok {
		t.Fatalf("stale sample should be rejected")
	}
}

func TestNearbyAlliesExcludeDeadAndStaleSamples(t *testing.T) {
	tl := &MatchTimeline{Players: map[string]*PlayerTimeline{
		"0": {PlayerSlot: 0, Team: dotaTeamRadiant, Samples: []HeroSample{{T: 20, X: 101, Y: 100, Alive: true}}},
		"1": {PlayerSlot: 1, Team: dotaTeamRadiant, Samples: []HeroSample{{T: 20, X: 100, Y: 100, Alive: false}}},
		"2": {PlayerSlot: 2, Team: dotaTeamRadiant, Samples: []HeroSample{{T: 20, X: 102, Y: 100, Alive: false}}},
		"3": {PlayerSlot: 3, Team: dotaTeamRadiant, Samples: []HeroSample{{T: 15.9, X: 103, Y: 100, Alive: true}}},
	}}
	got := nearbyAlliesAt(tl, dotaTeamRadiant, 1, 20, 100, 100)
	if len(got) != 1 || got[0].PlayerSlot != 0 {
		t.Fatalf("unexpected allies: %#v", got)
	}
}

func TestDamageContextIncludesWindowBoundariesAndExcludesFuture(t *testing.T) {
	damage := []DamageEvent{
		{T: 10, AttackerSlot: 128, VictimSlot: 1, Value: 100},
		{T: 15, AttackerSlot: 128, VictimSlot: 1, Value: 200},
		{T: 20, AttackerSlot: 1, VictimSlot: 128, Value: 300},
		{T: 20.01, AttackerSlot: 128, VictimSlot: 1, Value: 999},
	}
	r5, r10, d5, d10 := damageContextAt(damage, 1, 20)
	if r5 != 200 || r10 != 300 || d5 != 300 || d10 != 300 {
		t.Fatalf("unexpected windows: received=(%d,%d) dealt=(%d,%d)", r5, r10, d5, d10)
	}
}

func TestDeriveTargetDeathContextsSupportsDireTargetAndNeverSeenEnemy(t *testing.T) {
	targetSlot := 128
	tl := MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*PlayerTimeline{
			"0":   {PlayerSlot: 0, Team: dotaTeamRadiant},
			"128": {PlayerSlot: 128, Team: dotaTeamDire, Samples: []HeroSample{{T: 30, X: 100, Y: 100, Alive: false}}},
			"129": {PlayerSlot: 129, Team: dotaTeamDire},
		},
		Deaths: []DeathEvent{{T: 30, VictimSlot: &targetSlot}},
	}
	got := DeriveTargetDeathContexts(&tl)
	if len(got) != 1 || len(got[0].EnemyKnowledge) != 1 {
		t.Fatalf("unexpected contexts: %#v", got)
	}
	if got[0].EnemyKnowledge[0].PlayerSlot != 0 || got[0].EnemyKnowledge[0].Status != "never_seen" {
		t.Fatalf("unexpected enemy knowledge: %#v", got[0].EnemyKnowledge)
	}
}

func TestDeriveTargetDeathContextsHandlesMissingOptionalData(t *testing.T) {
	targetSlot := 1
	tl := MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*PlayerTimeline{
			"1": {PlayerSlot: 1, Team: dotaTeamRadiant},
		},
		Deaths: []DeathEvent{{T: 10, VictimSlot: &targetSlot}},
	}
	got := DeriveTargetDeathContexts(&tl)
	if len(got) != 1 {
		t.Fatalf("got %d contexts, want 1", len(got))
	}
	if got[0].PositionAvailable || got[0].Fight != nil || len(got[0].NearbyAllies) != 0 || len(got[0].EnemyKnowledge) != 0 {
		t.Fatalf("unexpected optional context: %#v", got[0])
	}
}
