package detector

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestChronosphereCombatContextAtUsesSameEnemyDamageRelationships(t *testing.T) {
	tl := &timeline.MatchTimeline{
		TargetPlayerSlot: 0,
		Players: map[string]*timeline.PlayerTimeline{
			"0":   {PlayerSlot: 0, Team: 2, HeroName: "npc_dota_hero_faceless_void"},
			"1":   {PlayerSlot: 1, Team: 2, HeroName: "npc_dota_hero_crystal_maiden"},
			"2":   {PlayerSlot: 2, Team: 2, HeroName: "npc_dota_hero_axe"},
			"128": {PlayerSlot: 128, Team: 3, HeroName: "npc_dota_hero_lina"},
			"129": {PlayerSlot: 129, Team: 3, HeroName: "npc_dota_hero_lion"},
			"130": {PlayerSlot: 130, Team: 3, HeroName: "npc_dota_hero_viper"},
		},
		Abilities: []timeline.AbilityEvent{{
			T: 100, PlayerSlot: 0, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_chronosphere",
		}},
		Damage: []timeline.DamageEvent{
			{T: 98.5, AttackerSlot: 128, VictimSlot: 0, Value: 100},
			{T: 99.0, AttackerSlot: 0, VictimSlot: 129, Value: 75},
			{T: 99.5, AttackerSlot: 1, VictimSlot: 128, Value: 50},
			{T: 99.8, AttackerSlot: 129, VictimSlot: 1, Value: 20},
			{T: 99.9, AttackerSlot: 2, VictimSlot: 130, Value: 999}, // unrelated enemy; do not count as same-enemy support
			{T: 100.5, AttackerSlot: 0, VictimSlot: 128, Value: 200},
			{T: 101.0, AttackerSlot: 0, VictimSlot: 129, Value: 300},
			{T: 101.5, AttackerSlot: 1, VictimSlot: 128, Value: 150},
			{T: 104.9, AttackerSlot: 1, VictimSlot: 129, Value: 50},
			{T: 104.0, AttackerSlot: 2, VictimSlot: 130, Value: 777}, // unrelated follow-up damage
			{T: 105.1, AttackerSlot: 0, VictimSlot: 128, Value: 999}, // outside fixed 5s follow-up window
		},
	}

	got, ok := ChronosphereCombatContextAt(tl, 100)
	if !ok {
		t.Fatal("ChronosphereCombatContextAt returned ok=false")
	}
	if got.FollowupWindowSeconds != 5 {
		t.Fatalf("followup window=%v, want 5", got.FollowupWindowSeconds)
	}
	if got.FollowupWindowEqualsSpellDuration || got.CaughtHeroesConfirmedFromReplay || got.CastPlacementConfirmedFromReplay {
		t.Fatalf("scope flags unexpectedly claim exact Chronosphere reconstruction: %#v", got)
	}
	if got.RecentEnemyInteractorsBeforeCast != 2 {
		t.Fatalf("recent enemy interactors=%d, want 2", got.RecentEnemyInteractorsBeforeCast)
	}
	if got.RecentAlliedTeammatesInteractingWithSameEnemies != 1 {
		t.Fatalf("recent allied same-enemy interactors=%d, want 1", got.RecentAlliedTeammatesInteractingWithSameEnemies)
	}
	if got.TargetEnemyHeroesDamagedInFollowup != 2 {
		t.Fatalf("target enemy heroes damaged=%d, want 2", got.TargetEnemyHeroesDamagedInFollowup)
	}
	if got.TargetHeroDamageInFollowup != 500 {
		t.Fatalf("target follow-up damage=%d, want 500", got.TargetHeroDamageInFollowup)
	}
	if got.AlliedTeammatesDamagingTargetVictimsInFollowup != 1 {
		t.Fatalf("allied contributors=%d, want 1", got.AlliedTeammatesDamagingTargetVictimsInFollowup)
	}
	if got.AlliedHeroDamageToTargetVictimsInFollowup != 200 {
		t.Fatalf("allied damage to target victims=%d, want 200", got.AlliedHeroDamageToTargetVictimsInFollowup)
	}
	if got.SecondsToFirstTargetHeroDamageAfterCast == nil || math.Abs(*got.SecondsToFirstTargetHeroDamageAfterCast-0.5) > 1e-9 {
		t.Fatalf("seconds to first target damage=%v, want 0.5", got.SecondsToFirstTargetHeroDamageAfterCast)
	}
}

func TestChronosphereCombatContextAtRequiresObservedTargetChronosphere(t *testing.T) {
	tl := &timeline.MatchTimeline{
		TargetPlayerSlot: 0,
		Players: map[string]*timeline.PlayerTimeline{
			"0": {PlayerSlot: 0, Team: 2, HeroName: "npc_dota_hero_faceless_void"},
		},
	}
	if _, ok := ChronosphereCombatContextAt(tl, 100); ok {
		t.Fatal("context unexpectedly accepted missing Chronosphere cast")
	}
}
