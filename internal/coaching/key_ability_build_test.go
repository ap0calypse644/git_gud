package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputAddsCompactChronosphereReviews(t *testing.T) {
	targetSlot := 2
	enemySlot := 129
	targetDeath := targetSlot
	enemyDeath := enemySlot
	tl := &timeline.MatchTimeline{
		MatchID:          8973449904,
		TargetPlayerSlot: targetSlot,
		Players: map[string]*timeline.PlayerTimeline{
			"2": {
				PlayerSlot: targetSlot,
				Team:       2,
				HeroName:   "npc_dota_hero_faceless_void",
				Samples: []timeline.HeroSample{
					{T: 99, X: 111.25, Y: 222.5, HP: 1400, MaxHP: 2000, Alive: true},
					{T: 199, X: 333.75, Y: 444.5, HP: 2000, MaxHP: 2000, Alive: true},
				},
			},
			"129": {
				PlayerSlot: enemySlot,
				Team:       3,
				HeroName:   "npc_dota_hero_legion_commander",
				Samples:    []timeline.HeroSample{{T: 99, X: 777.125, Y: 888.25, Alive: true}},
			},
		},
		Abilities: []timeline.AbilityEvent{
			{T: 100, PlayerSlot: targetSlot, Ability: "faceless_void_chronosphere"},
			{T: 200, PlayerSlot: targetSlot, Ability: "faceless_void_chronosphere"},
		},
		Items: []timeline.ItemEvent{
			{T: 99.667, PlayerSlot: enemySlot, Item: "blade_mail", Action: "use"},
		},
		Damage: []timeline.DamageEvent{
			{T: 102, AttackerSlot: targetSlot, VictimSlot: enemySlot, Value: 500},
			{T: 102.1, AttackerSlot: enemySlot, VictimSlot: targetSlot, Inflictor: "blade_mail", Value: 700},
			{T: 202, AttackerSlot: targetSlot, VictimSlot: enemySlot, Value: 900},
		},
		Deaths: []timeline.DeathEvent{
			{T: 103, VictimSlot: &targetDeath, Inflictor: "blade_mail"},
			{T: 204, VictimSlot: &enemyDeath, Inflictor: "faceless_void_time_lock"},
		},
	}

	got := BuildMatchCoachingInput(tl)
	if got.Hero != "faceless_void" {
		t.Fatalf("hero = %q, want faceless_void", got.Hero)
	}
	if got.MatchID != 8973449904 {
		t.Fatalf("match_id = %d", got.MatchID)
	}

	var reflectMoment, firstCastMoment, secondCastMoment *CoachingMoment
	for i := range got.Moments {
		moment := &got.Moments[i]
		switch {
		case moment.Type == detector.TypeActiveDamageReflectInteractionCandidate && moment.StartT == 100:
			reflectMoment = moment
		case moment.Type == detector.TypeKeyAbilityUseReviewCandidate && moment.StartT == 100:
			firstCastMoment = moment
		case moment.Type == detector.TypeKeyAbilityUseReviewCandidate && moment.StartT == 200:
			secondCastMoment = moment
		}
	}
	if reflectMoment == nil || firstCastMoment == nil || secondCastMoment == nil {
		t.Fatalf("missing expected key-ability moments: %#v", got.Moments)
	}

	reflect, ok := reflectMoment.Evidence.(ActiveDamageReflectReviewEvidence)
	if !ok {
		t.Fatalf("reflect evidence type = %T", reflectMoment.Evidence)
	}
	if reflect.Item != "blade_mail" || reflect.PlayerKnowledgeStatus != detector.PlayerKnowledgeNotConfirmedFromReplay || reflect.ReflectedDamageAfterCast != 700 || !reflect.TargetDeathToReflect {
		t.Fatalf("reflect evidence = %+v", reflect)
	}

	firstCast, ok := firstCastMoment.Evidence.(KeyAbilityReviewEvidence)
	if !ok {
		t.Fatalf("key ability evidence type = %T", firstCastMoment.Evidence)
	}
	if firstCast.Ability != "faceless_void_chronosphere" || firstCast.TargetDeathInflictor != "blade_mail" || firstCast.TargetDeathT == nil {
		t.Fatalf("first cast evidence = %+v", firstCast)
	}

	secondCast, ok := secondCastMoment.Evidence.(KeyAbilityReviewEvidence)
	if !ok {
		t.Fatalf("second key ability evidence type = %T", secondCastMoment.Evidence)
	}
	if secondCast.TargetDeathT != nil || secondCast.EnemyDeathsAfterCast != 1 || secondCast.TargetDamageDealtAfterCast != 900 {
		t.Fatalf("control cast evidence = %+v", secondCast)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal coaching input: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{
		`"players"`,
		`"samples"`,
		`"item_user_slot"`,
		"111.25",
		"222.5",
		"777.125",
		"888.25",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("key ability coaching input leaked %q: %s", forbidden, text)
		}
	}
}

func TestKeyAbilityCandidateEvidenceFailsClosed(t *testing.T) {
	badGeneric := detector.KeyAbilityCandidate{
		Type: detector.TypeKeyAbilityUseReviewCandidate,
		KeyAbility: &detector.KeyAbilityUseEvidence{
			Ability:              "faceless_void_chronosphere",
			CastT:                100,
			PreCastWindowSeconds: 3,
			OutcomeWindowSeconds: 8,
			TargetDeathT:         float64Ptr(99),
		},
	}
	if _, _, ok := keyAbilityCandidateEvidence(badGeneric); ok {
		t.Fatal("accepted retrospective death before cast")
	}

	badReflect := detector.KeyAbilityCandidate{
		Type: detector.TypeActiveDamageReflectInteractionCandidate,
		ActiveDamageReflect: &detector.ActiveDamageReflectEvidence{
			Ability:                  "faceless_void_chronosphere",
			CastT:                    100,
			Item:                     "blade_mail",
			ItemUseT:                 99.7,
			SecondsFromItemUseToCast: 0.3,
			PlayerKnowledgeStatus:    "known_visible",
			OutcomeWindowSeconds:     8,
			ReflectedDamageAfterCast: 500,
			FirstReflectedDamageT:    float64Ptr(102),
		},
	}
	if _, _, ok := keyAbilityCandidateEvidence(badReflect); ok {
		t.Fatal("accepted unsupported player-knowledge promotion")
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
