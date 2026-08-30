package detector

import "github.com/ap0calypse644/git_gud/internal/timeline"

const (
	TimeWalkPreDamageWindowSeconds  = 2.0
	TimeWalkOutcomeWindowSeconds    = 3.0
	TimeWalkMinRecentDamagePctMaxHP = 0.20
)

// appendTimeWalkDamageRecoveryCandidates keeps Time Walk review volume bounded
// by emitting only casts with a meaningful recent damage-backtrack opportunity.
// Candidate selection depends only on pre-cast facts. The compact provider
// boundary reuses KeyAbilityUseEvidence rather than creating a parallel schema.
func appendTimeWalkDamageRecoveryCandidates(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, out *KeyAbilityAnalysis) {
	if tl == nil || target == nil || out == nil || target.HeroName != "npc_dota_hero_faceless_void" {
		return
	}
	for _, cast := range tl.Abilities {
		if cast.PlayerSlot != target.PlayerSlot || cast.Ability != "faceless_void_time_walk" {
			continue
		}
		evidence, ok := buildTimeWalkDamageRecoveryEvidence(tl, target, cast)
		if !ok {
			continue
		}
		copied := evidence
		out.Candidates = append(out.Candidates, KeyAbilityCandidate{
			Type:       TypeKeyAbilityUseReviewCandidate,
			T:          cast.T,
			Confidence: ConfidenceLow,
			KeyAbility: &copied,
		})
	}
}

func buildTimeWalkDamageRecoveryEvidence(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, cast timeline.AbilityEvent) (KeyAbilityUseEvidence, bool) {
	preSample, ok := keyAbilityLatestSampleAtOrBefore(target.Samples, cast.T)
	if !ok || !preSample.Alive || preSample.MaxHP <= 0 || preSample.HP < 0 {
		return KeyAbilityUseEvidence{}, false
	}

	out := KeyAbilityUseEvidence{
		Hero:                       target.HeroName,
		Ability:                    cast.Ability,
		CastT:                      cast.T,
		TargetSampleAvailable:      true,
		TargetAliveAtCast:          preSample.Alive,
		TargetHPAtCast:             preSample.HP,
		TargetMaxHPAtCast:          preSample.MaxHP,
		AlliedTeammatesAliveAtCast: keyAbilityAliveTeammatesAt(tl, target, cast.T),
		PreCastWindowSeconds:       TimeWalkPreDamageWindowSeconds,
		OutcomeWindowSeconds:       TimeWalkOutcomeWindowSeconds,
	}
	hpPct := float64(preSample.HP) / float64(preSample.MaxHP)
	out.TargetHPPctAtCast = &hpPct

	preStart := cast.T - TimeWalkPreDamageWindowSeconds
	outcomeEnd := cast.T + TimeWalkOutcomeWindowSeconds
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

	if float64(out.TargetDamageReceivedBeforeCast)/float64(preSample.MaxHP) < TimeWalkMinRecentDamagePctMaxHP {
		return KeyAbilityUseEvidence{}, false
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
	return out, true
}
