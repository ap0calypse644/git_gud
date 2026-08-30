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
					// This future sample is retrospective and must not become cast-time HP.
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
	if candidate.Type != TypeTimeWalkDamageRecoveryReviewCandidate || candidate.TimeWalkDamageRecovery == nil {
		t.Fatalf("candidate = %+v", candidate)
	}
	evidence := candidate.TimeWalkDamageRecovery
	if evidence.Ability != "faceless_void_time_walk" || evidence.CastT != 100 {
		t.Fatalf("identity = %+v", evidence)
	}
	if !evidence.TargetSampleAvailable || !evidence.TargetAliveAtCast || evidence.TargetSampleT != 99.5 {
		t.Fatalf("pre-cast sample = %+v", evidence)
	}
	if evidence.TargetHPAtCastSample != 300 || evidence.TargetMaxHPAtCastSample != 1000 {
		t.Fatalf("future sample leaked into cast state: %+v", evidence)
	}
	if evidence.TargetHPPctAtCastSample == nil || math.Abs(*evidence.TargetHPPctAtCastSample-0.3) > 1e-9 {
		t.Fatalf("pre-cast hp pct = %v, want 0.3", evidence.TargetHPPctAtCastSample)
	}
	if math.Abs(evidence.TargetSampleAgeSeconds-0.5) > 1e-9 {
		t.Fatalf("sample age = %f, want 0.5", evidence.TargetSampleAgeSeconds)
	}
	if evidence.IncomingDamageBeforeCast != 250 || math.Abs(evidence.IncomingDamagePctMaxHP-0.25) > 1e-9 {
		t.Fatalf("pre-cast damage = %+v", evidence)
	}
	if !evidence.PostCastSampleAvailable || evidence.PostCastSampleT == nil || *evidence.PostCastSampleT != 100.1 {
		t.Fatalf("post-cast sample = %+v", evidence)
	}
	if evidence.PostCastSampleDelaySeconds == nil || math.Abs(*evidence.PostCastSampleDelaySeconds-0.1) > 1e-9 {
		t.Fatalf("post-cast sample delay = %v, want 0.1", evidence.PostCastSampleDelaySeconds)
	}
	if evidence.TargetHPAtPostCastSample != 900 || evidence.IncomingDamageAfterCast != 50 {
		t.Fatalf("retrospective evidence = %+v", evidence)
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
