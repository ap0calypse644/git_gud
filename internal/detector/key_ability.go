package detector

import (
	"math"
	"sort"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const (
	TypeKeyAbilityUseReviewCandidate             = "key_ability_use_review_candidate"
	TypeActiveDamageReflectInteractionCandidate = "active_damage_reflect_interaction_candidate"

	KeyAbilityPreCastWindowSeconds       = 3.0
	KeyAbilityOutcomeWindowSeconds       = 8.0
	RelevantItemUseLookbackSeconds       = 2.0
	PlayerKnowledgeNotConfirmedFromReplay = "not_confirmed_from_replay"
)

// KeyAbilityAnalysis exposes one compact review context per supported target
// key-ability cast plus narrower mechanic-specific interaction candidates when
// the replay provides direct evidence. These are review targets, not verdicts.
type KeyAbilityAnalysis struct {
	MatchID     int64                  `json:"match_id"`
	Assessments []KeyAbilityAssessment `json:"assessments"`
	Candidates  []KeyAbilityCandidate  `json:"candidates"`
}

type KeyAbilityAssessment struct {
	CastT               float64                       `json:"cast_t"`
	Ability             string                        `json:"ability"`
	Evidence            KeyAbilityUseEvidence         `json:"evidence"`
	ActiveDamageReflect *ActiveDamageReflectEvidence  `json:"active_damage_reflect,omitempty"`
}

type KeyAbilityCandidate struct {
	Type                string                       `json:"type"`
	T                   float64                      `json:"t"`
	Confidence          string                       `json:"confidence"`
	KeyAbility          *KeyAbilityUseEvidence       `json:"key_ability,omitempty"`
	ActiveDamageReflect *ActiveDamageReflectEvidence `json:"active_damage_reflect,omitempty"`
}

// KeyAbilityUseEvidence deliberately separates decision-time state from a fixed
// retrospective outcome window. The outcome window is for review prioritization
// only; it must not be used to invent what the player knew when casting.
type KeyAbilityUseEvidence struct {
	Hero                              string   `json:"hero"`
	Ability                           string   `json:"ability"`
	CastT                             float64  `json:"cast_t"`
	TargetSampleAvailable             bool     `json:"target_sample_available"`
	TargetAliveAtCast                 bool     `json:"target_alive_at_cast"`
	TargetHPAtCast                    int32    `json:"target_hp_at_cast,omitempty"`
	TargetMaxHPAtCast                 int32    `json:"target_max_hp_at_cast,omitempty"`
	TargetHPPctAtCast                 *float64 `json:"target_hp_pct_at_cast,omitempty"`
	AlliedTeammatesAliveAtCast        int      `json:"allied_teammates_alive_at_cast"`
	PreCastWindowSeconds              float64  `json:"pre_cast_window_seconds"`
	TargetDamageDealtBeforeCast       int64    `json:"target_damage_dealt_before_cast"`
	TargetDamageReceivedBeforeCast    int64    `json:"target_damage_received_before_cast"`
	OutcomeWindowSeconds              float64  `json:"outcome_window_seconds"`
	TargetDamageDealtAfterCast        int64    `json:"target_damage_dealt_after_cast"`
	TargetDamageReceivedAfterCast     int64    `json:"target_damage_received_after_cast"`
	EnemyDeathsAfterCast              int      `json:"enemy_deaths_after_cast"`
	AlliedDeathsAfterCast             int      `json:"allied_deaths_after_cast"`
	TargetDeathT                      *float64 `json:"target_death_t,omitempty"`
	TargetDeathInflictor              string   `json:"target_death_inflictor,omitempty"`
}

// ActiveDamageReflectEvidence is a narrow mechanic-level interaction. The item
// use is a replay event before the cast, but PlayerKnowledgeStatus explicitly
// prevents downstream layers from silently promoting that replay truth into
// confirmed player knowledge. Reflected damage and death are retrospective.
type ActiveDamageReflectEvidence struct {
	Ability                    string   `json:"ability"`
	CastT                      float64  `json:"cast_t"`
	Item                       string   `json:"item"`
	ItemUserSlot               int      `json:"item_user_slot"`
	ItemUseT                   float64  `json:"item_use_t"`
	SecondsFromItemUseToCast   float64  `json:"seconds_from_item_use_to_cast"`
	PlayerKnowledgeStatus      string   `json:"player_knowledge_status"`
	OutcomeWindowSeconds       float64  `json:"outcome_window_seconds"`
	ReflectedDamageAfterCast   int64    `json:"reflected_damage_after_cast"`
	FirstReflectedDamageT      *float64 `json:"first_reflected_damage_t,omitempty"`
	TargetDeathToReflect       bool     `json:"target_death_to_reflect"`
	TargetDeathT               *float64 `json:"target_death_t,omitempty"`
}

// AnalyzeKeyAbilities currently supports Faceless Void's Chronosphere as the
// first real-replay calibration fixture. The mechanics are implemented through
// reusable event relationships rather than hero-vs-hero special cases.
func AnalyzeKeyAbilities(tl *timeline.MatchTimeline) KeyAbilityAnalysis {
	out := KeyAbilityAnalysis{Assessments: []KeyAbilityAssessment{}, Candidates: []KeyAbilityCandidate{}}
	if tl == nil {
		return out
	}
	out.MatchID = tl.MatchID

	target := keyAbilityPlayerForSlot(tl, tl.TargetPlayerSlot)
	if target == nil {
		return out
	}

	for _, cast := range tl.Abilities {
		if cast.PlayerSlot != tl.TargetPlayerSlot || !supportedKeyAbility(target.HeroName, cast.Ability) {
			continue
		}

		evidence := buildKeyAbilityEvidence(tl, target, cast)
		assessment := KeyAbilityAssessment{
			CastT:    cast.T,
			Ability:  cast.Ability,
			Evidence: evidence,
		}
		out.Candidates = append(out.Candidates, KeyAbilityCandidate{
			Type:       TypeKeyAbilityUseReviewCandidate,
			T:          cast.T,
			Confidence: ConfidenceLow,
			KeyAbility: &evidence,
		})

		if reflect := activeDamageReflectInteraction(tl, target, cast); reflect != nil {
			assessment.ActiveDamageReflect = reflect
			copied := *reflect
			out.Candidates = append(out.Candidates, KeyAbilityCandidate{
				Type:                TypeActiveDamageReflectInteractionCandidate,
				T:                   cast.T,
				Confidence:          ConfidenceLow,
				ActiveDamageReflect: &copied,
			})
		}
		out.Assessments = append(out.Assessments, assessment)
	}

	sort.SliceStable(out.Assessments, func(i, j int) bool {
		return out.Assessments[i].CastT < out.Assessments[j].CastT
	})
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].T == out.Candidates[j].T {
			return out.Candidates[i].Type < out.Candidates[j].Type
		}
		return out.Candidates[i].T < out.Candidates[j].T
	})
	return out
}

func supportedKeyAbility(hero, ability string) bool {
	switch hero {
	case "npc_dota_hero_faceless_void":
		return ability == "faceless_void_chronosphere"
	default:
		return false
	}
}

func buildKeyAbilityEvidence(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, cast timeline.AbilityEvent) KeyAbilityUseEvidence {
	out := KeyAbilityUseEvidence{
		Hero:                       target.HeroName,
		Ability:                    cast.Ability,
		CastT:                      cast.T,
		AlliedTeammatesAliveAtCast: keyAbilityAliveTeammatesAt(tl, target, cast.T),
		PreCastWindowSeconds:       KeyAbilityPreCastWindowSeconds,
		OutcomeWindowSeconds:       KeyAbilityOutcomeWindowSeconds,
	}

	if sample, ok := keyAbilityLatestSampleAtOrBefore(target.Samples, cast.T); ok {
		out.TargetSampleAvailable = true
		out.TargetAliveAtCast = sample.Alive
		out.TargetHPAtCast = sample.HP
		out.TargetMaxHPAtCast = sample.MaxHP
		if sample.MaxHP > 0 {
			pct := float64(sample.HP) / float64(sample.MaxHP)
			out.TargetHPPctAtCast = &pct
		}
	}

	preStart := cast.T - KeyAbilityPreCastWindowSeconds
	outcomeEnd := cast.T + KeyAbilityOutcomeWindowSeconds
	for _, damage := range tl.Damage {
		switch {
		case damage.T >= preStart && damage.T <= cast.T:
			if damage.AttackerSlot == target.PlayerSlot {
				out.TargetDamageDealtBeforeCast += int64(damage.Value)
			}
			if damage.VictimSlot == target.PlayerSlot {
				out.TargetDamageReceivedBeforeCast += int64(damage.Value)
			}
		case damage.T > cast.T && damage.T <= outcomeEnd:
			if damage.AttackerSlot == target.PlayerSlot {
				out.TargetDamageDealtAfterCast += int64(damage.Value)
			}
			if damage.VictimSlot == target.PlayerSlot {
				out.TargetDamageReceivedAfterCast += int64(damage.Value)
			}
		}
	}

	for _, death := range tl.Deaths {
		if death.T <= cast.T || death.T > outcomeEnd || death.VictimSlot == nil {
			continue
		}
		victim := keyAbilityPlayerForSlot(tl, *death.VictimSlot)
		if victim == nil {
			continue
		}
		if victim.PlayerSlot == target.PlayerSlot && out.TargetDeathT == nil {
			t := death.T
			out.TargetDeathT = &t
			out.TargetDeathInflictor = death.Inflictor
		}
		if victim.Team == target.Team {
			out.AlliedDeathsAfterCast++
		} else {
			out.EnemyDeathsAfterCast++
		}
	}
	return out
}

func activeDamageReflectInteraction(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, cast timeline.AbilityEvent) *ActiveDamageReflectEvidence {
	outcomeEnd := cast.T + KeyAbilityOutcomeWindowSeconds
	for _, item := range tl.Items {
		if item.Action != "use" || item.Item != "blade_mail" || item.T > cast.T || cast.T-item.T > RelevantItemUseLookbackSeconds {
			continue
		}
		itemUser := keyAbilityPlayerForSlot(tl, item.PlayerSlot)
		if itemUser == nil || itemUser.Team == target.Team {
			continue
		}

		var reflected int64
		var firstReflected *float64
		for _, damage := range tl.Damage {
			if damage.T <= cast.T || damage.T > outcomeEnd || damage.AttackerSlot != item.PlayerSlot || damage.VictimSlot != target.PlayerSlot || damage.Inflictor != "blade_mail" {
				continue
			}
			reflected += int64(damage.Value)
			if firstReflected == nil {
				t := damage.T
				firstReflected = &t
			}
		}
		if reflected <= 0 {
			continue
		}

		evidence := &ActiveDamageReflectEvidence{
			Ability:                  cast.Ability,
			CastT:                    cast.T,
			Item:                     item.Item,
			ItemUserSlot:             item.PlayerSlot,
			ItemUseT:                 item.T,
			SecondsFromItemUseToCast: cast.T - item.T,
			PlayerKnowledgeStatus:    PlayerKnowledgeNotConfirmedFromReplay,
			OutcomeWindowSeconds:     KeyAbilityOutcomeWindowSeconds,
			ReflectedDamageAfterCast: reflected,
			FirstReflectedDamageT:    firstReflected,
		}
		for _, death := range tl.Deaths {
			if death.T <= cast.T || death.T > outcomeEnd || death.VictimSlot == nil || *death.VictimSlot != target.PlayerSlot {
				continue
			}
			if death.Inflictor == "blade_mail" {
				evidence.TargetDeathToReflect = true
				t := death.T
				evidence.TargetDeathT = &t
				break
			}
		}
		return evidence
	}
	return nil
}

func keyAbilityAliveTeammatesAt(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, t float64) int {
	count := 0
	for _, player := range tl.Players {
		if player == nil || player.PlayerSlot == target.PlayerSlot || player.Team != target.Team {
			continue
		}
		if sample, ok := keyAbilityLatestSampleAtOrBefore(player.Samples, t); ok && sample.Alive {
			count++
		}
	}
	return count
}

func keyAbilityLatestSampleAtOrBefore(samples []timeline.HeroSample, t float64) (timeline.HeroSample, bool) {
	var best timeline.HeroSample
	found := false
	for _, sample := range samples {
		if sample.T > t || math.IsNaN(sample.T) || math.IsInf(sample.T, 0) {
			continue
		}
		if !found || sample.T > best.T {
			best = sample
			found = true
		}
	}
	return best, found
}

func keyAbilityPlayerForSlot(tl *timeline.MatchTimeline, slot int) *timeline.PlayerTimeline {
	if tl == nil {
		return nil
	}
	for _, player := range tl.Players {
		if player != nil && player.PlayerSlot == slot {
			return player
		}
	}
	return nil
}
