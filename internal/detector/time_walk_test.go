package detector

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAnalyzeKeyAbilitiesTimeWalkDamageRecoveryUsesNormalizedBurstAndCausalSample(t *testing.T) {
	targetSlot := 2
	tl := &timeline.MatchTimeline{
		MatchID:          8973449904,
		TargetPlayerSlot: targetSlot,
		Players: map[string]*timeline.PlayerTimeline{
			"2": {
				PlayerSlot: targetSlot,
				Team:       2,
				HeroName:   "npc_dota_hero_faceless_void",
				Samples: []timeline.HeroSample{
					{T: 99.5, HP: 300, MaxHP: 1000, Alive: true},
					// This future sample must not become cast-time state.
					{T: 100.1, HP: 900, MaxHP: 1000, Alive: true},
					{T: 199.5, HP: 1000, MaxHP: 2000, Alive: true},
					{T: 200.1, HP: 1300, MaxHP: 2000, Alive: true},
				},
			},
		},
		Abilities: []timeline.AbilityEvent{
			{T: 100, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_time_walk"},
			{T: 200, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_time_walk"},
		},
		Damage: []timeline.DamageEvent{
			{T: 98.5, AttackerSlot: 129, VictimSlot: targetSlot, Value: 250},
			{T: 101, AttackerSlot: 129, VictimSlot: targetSlot, Value: 50},
			// Larger absolute damage, but only 17.5% of this cast's max HP.
			{T: 198.5, AttackerSlot: 129, VictimSlot: targetSlot, Value: 350},
		},
	}

	analysis := AnalyzeKeyAbilities(tl)
	if got := len(analysis.Assessments); got != 0 {
		t.Fatalf("generic assessments = %d, want 0 for Time Walk-only fixture", got)
	}
	if got, want := len(analysis.Candidates), 1; got != want {
		t.Fatalf("candidates = %d, want %d", got, want)
	}

	candidate := analysis.Candidates[0]
	if candidate.Type != TypeKeyAbilityUseReviewCandidate || candidate.KeyAbility == nil {
		t.Fatalf("candidate = %+v", candidate)
	}
	evidence := candidate.KeyAbility
	if evidence.Ability != "faceless_void_time_walk" || evidence.CastT != 100 {
		t.Fatalf("identity = %+v", evidence)
	}
	if evidence.PreCastWindowSeconds != TimeWalkPreDamageWindowSeconds || evidence.OutcomeWindowSeconds != TimeWalkOutcomeWindowSeconds {
		t.Fatalf("windows = %+v", evidence)
	}
	if !evidence.TargetSampleAvailable || !evidence.TargetAliveAtCast {
		t.Fatalf("pre-cast sample = %+v", evidence)
	}
	if evidence.TargetHPAtCast != 300 || evidence.TargetMaxHPAtCast != 1000 {
		t.Fatalf("future sample leaked into cast state: %+v", evidence)
	}
	if evidence.TargetHPPctAtCast == nil || math.Abs(*evidence.TargetHPPctAtCast-0.3) > 1e-9 {
		t.Fatalf("pre-cast hp pct = %v, want 0.3", evidence.TargetHPPctAtCast)
	}
	if evidence.TargetDamageReceivedBeforeCast != 250 || evidence.TargetDamageReceivedAfterCast != 50 {
		t.Fatalf("damage evidence = %+v", evidence)
	}
}

func TestTimeWalkDamageRecoveryRequiresFacelessVoidAndRecentDamageThreshold(t *testing.T) {
	for _, tc := range []struct {
		name string
		hero string
		dmg  int32
	}{
		{name: "below normalized threshold", hero: "npc_dota_hero_faceless_void", dmg: 199},
		{name: "unsupported hero", hero: "npc_dota_hero_antimage", dmg: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targetSlot := 2
			tl := &timeline.MatchTimeline{
				TargetPlayerSlot: targetSlot,
				Players: map[string]*timeline.PlayerTimeline{
					"2": {
						PlayerSlot: targetSlot,
						Team:       2,
						HeroName:   tc.hero,
						Samples:    []timeline.HeroSample{{T: 99.5, HP: 500, MaxHP: 1000, Alive: true}},
					},
				},
				Abilities: []timeline.AbilityEvent{{T: 100, PlayerSlot: targetSlot, Ability: "faceless_void_time_walk"}},
				Damage:    []timeline.DamageEvent{{T: 99, VictimSlot: targetSlot, Value: tc.dmg}},
			}
			analysis := AnalyzeKeyAbilities(tl)
			if len(analysis.Candidates) != 0 {
				t.Fatalf("unexpected candidates = %+v", analysis.Candidates)
			}
		})
	}
}
