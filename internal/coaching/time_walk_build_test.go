package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputAddsCompactTimeWalkDamageRecoveryReview(t *testing.T) {
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
					{T: 99.5, X: 111.25, Y: 222.5, HP: 300, MaxHP: 1000, Alive: true},
					{T: 100.1, X: 333.75, Y: 444.5, HP: 900, MaxHP: 1000, Alive: true},
				},
			},
			"129": {
				PlayerSlot: 129,
				Team:       3,
				HeroName:   "npc_dota_hero_axe",
				Samples:    []timeline.HeroSample{{T: 99.5, X: 777.125, Y: 888.25, Alive: true}},
			},
		},
		Abilities: []timeline.AbilityEvent{
			{T: 100, PlayerSlot: targetSlot, Hero: "npc_dota_hero_faceless_void", Ability: "faceless_void_time_walk"},
		},
		Damage: []timeline.DamageEvent{
			{T: 98.5, AttackerSlot: 129, VictimSlot: targetSlot, Value: 250},
			{T: 101, AttackerSlot: 129, VictimSlot: targetSlot, Value: 50},
		},
	}

	input := BuildMatchCoachingInput(tl)
	if input.Hero != "faceless_void" {
		t.Fatalf("hero = %q", input.Hero)
	}
	if got, want := len(input.Moments), 1; got != want {
		t.Fatalf("moments = %d, want %d: %+v", got, want, input.Moments)
	}
	moment := input.Moments[0]
	if moment.Type != detector.TypeKeyAbilityUseReviewCandidate || moment.StartT != 100 || moment.EndT != 100 {
		t.Fatalf("moment = %+v", moment)
	}
	evidence, ok := moment.Evidence.(KeyAbilityReviewEvidence)
	if !ok {
		t.Fatalf("evidence type = %T", moment.Evidence)
	}
	if evidence.Ability != "faceless_void_time_walk" || evidence.PreCastWindowSeconds != detector.TimeWalkPreDamageWindowSeconds {
		t.Fatalf("Time Walk evidence = %+v", evidence)
	}
	if evidence.TargetHPAtCast != 300 || evidence.TargetMaxHPAtCast != 1000 || evidence.TargetDamageReceivedBeforeCast != 250 {
		t.Fatalf("decision-time evidence = %+v", evidence)
	}
	if evidence.OutcomeWindowSeconds != detector.TimeWalkOutcomeWindowSeconds || evidence.TargetDamageReceivedAfterCast != 50 {
		t.Fatalf("retrospective evidence = %+v", evidence)
	}

	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"111.25", "222.5", "333.75", "444.5", "777.125", "888.25", `"players"`, `"samples"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("compact Time Walk input leaked %q: %s", forbidden, text)
		}
	}
}
