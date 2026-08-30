package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputAddsCompactChronosphereFollowupWithoutIdentityLeakage(t *testing.T) {
	tl := &timeline.MatchTimeline{
		MatchID:          123,
		TargetPlayerSlot: 0,
		Players: map[string]*timeline.PlayerTimeline{
			"0": {
				PlayerSlot: 0, Team: 2, HeroName: "npc_dota_hero_faceless_void",
				Samples: []timeline.HeroSample{{T: 99.9, X: 777.125, Y: 888.25, HP: 800, MaxHP: 1000, Alive: true}},
			},
			"1": {
				PlayerSlot: 1, Team: 2, HeroName: "npc_dota_hero_crystal_maiden",
				Samples: []timeline.HeroSample{{T: 99.9, X: 111.5, Y: 112.5, HP: 900, MaxHP: 900, Alive: true}},
			},
			"128": {
				PlayerSlot: 128, Team: 3, HeroName: "npc_dota_hero_lina",
				Samples: []timeline.HeroSample{{T: 99.9, X: 120.5, Y: 121.5, HP: 900, MaxHP: 900, Alive: true}},
			},
			"129": {
				PlayerSlot: 129, Team: 3, HeroName: "npc_dota_hero_lion",
				Samples: []timeline.HeroSample{{T: 99.9, X: 122.5, Y: 123.5, HP: 900, MaxHP: 900, Alive: true}},
			},
		},
		Abilities: []timeline.AbilityEvent{{
			T: 100, PlayerSlot: 0, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_chronosphere",
		}},
		Damage: []timeline.DamageEvent{
			{T: 99.0, AttackerSlot: 128, VictimSlot: 0, Value: 100},
			{T: 99.5, AttackerSlot: 1, VictimSlot: 128, Value: 50},
			{T: 100.5, AttackerSlot: 0, VictimSlot: 128, Value: 200},
			{T: 101.0, AttackerSlot: 0, VictimSlot: 129, Value: 300},
			{T: 101.5, AttackerSlot: 1, VictimSlot: 128, Value: 150},
		},
	}

	input := BuildMatchCoachingInput(tl)
	var found *CoachingMoment
	for i := range input.Moments {
		moment := &input.Moments[i]
		if moment.Type != detector.TypeKeyAbilityUseReviewCandidate {
			continue
		}
		evidence, ok := moment.Evidence.(ChronosphereKeyAbilityReviewEvidence)
		if !ok || evidence.Ability != "faceless_void_chronosphere" {
			continue
		}
		found = moment
		break
	}
	if found == nil {
		t.Fatal("missing compact Chronosphere coaching moment")
	}

	evidence := found.Evidence.(ChronosphereKeyAbilityReviewEvidence)
	if evidence.TargetHPAtCast != 800 || evidence.TargetMaxHPAtCast != 1000 {
		t.Fatalf("generic sampled HP=%d/%d, want 800/1000", evidence.TargetHPAtCast, evidence.TargetMaxHPAtCast)
	}
	followup := evidence.ChronosphereFollowup
	if followup.FollowupWindowSeconds != 5 || followup.FollowupWindowEqualsSpellDuration {
		t.Fatalf("follow-up scope=%#v", followup)
	}
	if followup.CaughtHeroesConfirmedFromReplay || followup.CastPlacementConfirmedFromReplay {
		t.Fatalf("follow-up evidence claims unsupported reconstruction: %#v", followup)
	}
	if followup.RecentEnemyInteractorsBeforeCast != 1 || followup.RecentAlliedTeammatesInteractingWithSameEnemies != 1 {
		t.Fatalf("pre-cast relationship counts=%#v", followup)
	}
	if followup.TargetEnemyHeroesDamagedInFollowup != 2 || followup.TargetHeroDamageInFollowup != 500 {
		t.Fatalf("target follow-up=%#v", followup)
	}
	if followup.AlliedTeammatesDamagingTargetVictimsInFollowup != 1 || followup.AlliedHeroDamageToTargetVictimsInFollowup != 150 {
		t.Fatalf("allied follow-up=%#v", followup)
	}
	if followup.SecondsToFirstTargetHeroDamageAfterCast == nil || *followup.SecondsToFirstTargetHeroDamageAfterCast != 0.5 {
		t.Fatalf("first follow-up delay=%v, want 0.5", followup.SecondsToFirstTargetHeroDamageAfterCast)
	}

	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal compact input: %v", err)
	}
	text := string(encoded)
	for _, required := range []string{
		"\"chronosphere_followup\"",
		"\"caught_heroes_confirmed_from_replay\":false",
		"\"cast_placement_confirmed_from_replay\":false",
		"\"target_enemy_heroes_damaged_in_followup\":2",
		"\"allied_hero_damage_to_target_victims_in_followup\":150",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("compact input missing %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{"777.125", "888.25", "\"players\"", "\"samples\"", "player_slot", "victim_slot", "attacker_slot"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("compact input leaked %q: %s", forbidden, text)
		}
	}
}
