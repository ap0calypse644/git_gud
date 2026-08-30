package detector

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAnalyzeKeyAbilitiesChronosphereAndBladeMail(t *testing.T) {
	targetSlot := 2
	enemySlot := 129
	allySlot := 1
	tl := &timeline.MatchTimeline{
		MatchID:          8973449904,
		TargetPlayerSlot: targetSlot,
		Players: map[string]*timeline.PlayerTimeline{
			"2": {
				PlayerSlot: targetSlot,
				Team:       2,
				HeroName:   "npc_dota_hero_faceless_void",
				Samples: []timeline.HeroSample{
					{T: 99, HP: 1400, MaxHP: 2000, Alive: true},
					// Future state must not leak backward into the cast snapshot.
					{T: 101, HP: 100, MaxHP: 2000, Alive: true},
					{T: 199, HP: 2000, MaxHP: 2000, Alive: true},
				},
			},
			"1": {
				PlayerSlot: allySlot,
				Team:       2,
				HeroName:   "npc_dota_hero_crystal_maiden",
				Samples:    []timeline.HeroSample{{T: 99, Alive: true}, {T: 199, Alive: true}},
			},
			"129": {
				PlayerSlot: enemySlot,
				Team:       3,
				HeroName:   "npc_dota_hero_legion_commander",
				Samples:    []timeline.HeroSample{{T: 99, Alive: true}, {T: 199, Alive: true}},
			},
		},
		Abilities: []timeline.AbilityEvent{
			{T: 100, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_chronosphere"},
			{T: 150, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_time_walk"},
			{T: 200, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_chronosphere"},
		},
		Items: []timeline.ItemEvent{
			{T: 99.667, PlayerSlot: enemySlot, Hero: "npc_dota_hero_legion_commander", Item: "blade_mail", Action: "use"},
		},
		Damage: []timeline.DamageEvent{
			{T: 99, AttackerSlot: enemySlot, VictimSlot: targetSlot, Value: 100},
			{T: 102, AttackerSlot: targetSlot, VictimSlot: enemySlot, Value: 500},
			{T: 102.1, AttackerSlot: enemySlot, VictimSlot: targetSlot, Inflictor: "blade_mail", Value: 700},
			{T: 202, AttackerSlot: targetSlot, VictimSlot: enemySlot, Value: 800},
		},
		Deaths: []timeline.DeathEvent{
			{T: 103, VictimSlot: keyAbilityIntPtr(targetSlot), AttackerSlot: keyAbilityIntPtr(enemySlot), Inflictor: "blade_mail"},
			{T: 204, VictimSlot: keyAbilityIntPtr(enemySlot), AttackerSlot: keyAbilityIntPtr(targetSlot), Inflictor: "faceless_void_time_lock"},
		},
	}

	analysis := AnalyzeKeyAbilities(tl)
	if got, want := len(analysis.Assessments), 2; got != want {
		t.Fatalf("assessments = %d, want %d", got, want)
	}
	if got, want := len(analysis.Candidates), 3; got != want {
		t.Fatalf("candidates = %d, want %d", got, want)
	}

	first := analysis.Assessments[0]
	if first.Ability != "faceless_void_chronosphere" || first.CastT != 100 {
		t.Fatalf("first assessment = %+v", first)
	}
	if !first.Evidence.TargetSampleAvailable || first.Evidence.TargetHPAtCast != 1400 {
		t.Fatalf("cast sample used future state: %+v", first.Evidence)
	}
	if first.Evidence.TargetHPPctAtCast == nil || math.Abs(*first.Evidence.TargetHPPctAtCast-0.7) > 1e-9 {
		t.Fatalf("hp pct = %v, want 0.7", first.Evidence.TargetHPPctAtCast)
	}
	if first.Evidence.AlliedTeammatesAliveAtCast != 1 {
		t.Fatalf("allied teammates alive = %d, want 1", first.Evidence.AlliedTeammatesAliveAtCast)
	}
	if first.Evidence.TargetDamageReceivedBeforeCast != 100 || first.Evidence.TargetDamageDealtAfterCast != 500 {
		t.Fatalf("damage evidence = %+v", first.Evidence)
	}
	if first.Evidence.TargetDeathT == nil || *first.Evidence.TargetDeathT != 103 || first.Evidence.TargetDeathInflictor != "blade_mail" {
		t.Fatalf("target death evidence = %+v", first.Evidence)
	}
	if first.ActiveDamageReflect == nil {
		t.Fatal("expected active damage reflect interaction")
	}
	reflect := first.ActiveDamageReflect
	if reflect.Item != "blade_mail" || reflect.ItemUserSlot != enemySlot {
		t.Fatalf("reflect item evidence = %+v", reflect)
	}
	if math.Abs(reflect.SecondsFromItemUseToCast-0.333) > 0.001 {
		t.Fatalf("seconds item use to cast = %f, want ~0.333", reflect.SecondsFromItemUseToCast)
	}
	if reflect.PlayerKnowledgeStatus != PlayerKnowledgeNotConfirmedFromReplay {
		t.Fatalf("knowledge status = %q", reflect.PlayerKnowledgeStatus)
	}
	if reflect.ReflectedDamageAfterCast != 700 || !reflect.TargetDeathToReflect {
		t.Fatalf("reflect outcome = %+v", reflect)
	}

	second := analysis.Assessments[1]
	if second.ActiveDamageReflect != nil {
		t.Fatalf("reasonable control unexpectedly got reflect interaction: %+v", second.ActiveDamageReflect)
	}
	if second.Evidence.EnemyDeathsAfterCast != 1 || second.Evidence.TargetDamageDealtAfterCast != 800 || second.Evidence.TargetDeathT != nil {
		t.Fatalf("second cast control evidence = %+v", second.Evidence)
	}

	if analysis.Candidates[0].Type != TypeActiveDamageReflectInteractionCandidate || analysis.Candidates[1].Type != TypeKeyAbilityUseReviewCandidate {
		t.Fatalf("same-time candidate ordering = %q, %q", analysis.Candidates[0].Type, analysis.Candidates[1].Type)
	}
}

func TestAnalyzeKeyAbilitiesRequiresPreCastItemUseAndDirectReflectDamage(t *testing.T) {
	targetSlot := 2
	enemySlot := 129
	tl := &timeline.MatchTimeline{
		TargetPlayerSlot: targetSlot,
		Players: map[string]*timeline.PlayerTimeline{
			"2": {PlayerSlot: targetSlot, Team: 2, HeroName: "npc_dota_hero_faceless_void"},
			"129": {PlayerSlot: enemySlot, Team: 3, HeroName: "npc_dota_hero_legion_commander"},
		},
		Abilities: []timeline.AbilityEvent{
			{T: 100, PlayerSlot: targetSlot, Ability: "faceless_void_chronosphere"},
		},
		Items: []timeline.ItemEvent{
			// This activation is after the cast and cannot establish the interaction.
			{T: 100.2, PlayerSlot: enemySlot, Item: "blade_mail", Action: "use"},
		},
		Damage: []timeline.DamageEvent{
			{T: 101, AttackerSlot: enemySlot, VictimSlot: targetSlot, Inflictor: "blade_mail", Value: 500},
		},
		Deaths: []timeline.DeathEvent{
			{T: 102, VictimSlot: keyAbilityIntPtr(targetSlot), Inflictor: "blade_mail"},
		},
	}

	analysis := AnalyzeKeyAbilities(tl)
	if got, want := len(analysis.Candidates), 1; got != want {
		t.Fatalf("candidates = %d, want only generic key ability review", got)
	}
	if analysis.Candidates[0].Type != TypeKeyAbilityUseReviewCandidate {
		t.Fatalf("candidate type = %q", analysis.Candidates[0].Type)
	}
	if analysis.Assessments[0].ActiveDamageReflect != nil {
		t.Fatalf("future item use leaked into reflect interaction: %+v", analysis.Assessments[0].ActiveDamageReflect)
	}
}

func keyAbilityIntPtr(v int) *int {
	return &v
}
