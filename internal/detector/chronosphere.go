package detector

import "github.com/ap0calypse644/git_gud/internal/timeline"

const ChronosphereImmediateFollowupWindowSeconds = 5.0

// ChronosphereCombatContextEvidence adds damage-relationship context around an
// observed Chronosphere cast without claiming exact sphere placement or which
// heroes were caught. The 5-second follow-up window is a fixed review window,
// not a reconstructed spell duration.
type ChronosphereCombatContextEvidence struct {
	FollowupWindowSeconds                              float64  `json:"followup_window_seconds"`
	FollowupWindowEqualsSpellDuration                  bool     `json:"followup_window_equals_spell_duration"`
	CaughtHeroesConfirmedFromReplay                    bool     `json:"caught_heroes_confirmed_from_replay"`
	CastPlacementConfirmedFromReplay                   bool     `json:"cast_placement_confirmed_from_replay"`
	RecentEnemyInteractorsBeforeCast                   int      `json:"recent_enemy_interactors_before_cast"`
	RecentAlliedTeammatesInteractingWithSameEnemies    int      `json:"recent_allied_teammates_interacting_with_same_enemies_before_cast"`
	TargetEnemyHeroesDamagedInFollowup                 int      `json:"target_enemy_heroes_damaged_in_followup"`
	TargetHeroDamageInFollowup                         int64    `json:"target_hero_damage_in_followup"`
	AlliedTeammatesDamagingTargetVictimsInFollowup     int      `json:"allied_teammates_damaging_target_victims_in_followup"`
	AlliedHeroDamageToTargetVictimsInFollowup          int64    `json:"allied_hero_damage_to_target_victims_in_followup"`
	SecondsToFirstTargetHeroDamageAfterCast             *float64 `json:"seconds_to_first_target_hero_damage_after_cast,omitempty"`
}

// ChronosphereCombatContextAt derives compact context from the durable timeline
// only. Enemy identities and player slots are used internally for set matching
// and are intentionally not exposed in the returned evidence.
func ChronosphereCombatContextAt(tl *timeline.MatchTimeline, castT float64) (ChronosphereCombatContextEvidence, bool) {
	out := ChronosphereCombatContextEvidence{
		FollowupWindowSeconds:             ChronosphereImmediateFollowupWindowSeconds,
		FollowupWindowEqualsSpellDuration: false,
		CaughtHeroesConfirmedFromReplay:   false,
		CastPlacementConfirmedFromReplay:  false,
	}
	if tl == nil {
		return out, false
	}
	target := keyAbilityPlayerForSlot(tl, tl.TargetPlayerSlot)
	if target == nil || target.HeroName != "npc_dota_hero_faceless_void" {
		return out, false
	}

	observedCast := false
	for _, ability := range tl.Abilities {
		if ability.PlayerSlot == target.PlayerSlot && ability.Ability == "faceless_void_chronosphere" && ability.T == castT {
			observedCast = true
			break
		}
	}
	if !observedCast {
		return out, false
	}

	preStart := castT - KeyAbilityPreCastWindowSeconds
	recentEnemies := make(map[int]struct{})
	for _, damage := range tl.Damage {
		if damage.T < preStart || damage.T > castT {
			continue
		}
		switch {
		case damage.AttackerSlot == target.PlayerSlot:
			if victim := keyAbilityPlayerForSlot(tl, damage.VictimSlot); victim != nil && victim.Team != target.Team {
				recentEnemies[victim.PlayerSlot] = struct{}{}
			}
		case damage.VictimSlot == target.PlayerSlot:
			if attacker := keyAbilityPlayerForSlot(tl, damage.AttackerSlot); attacker != nil && attacker.Team != target.Team {
				recentEnemies[attacker.PlayerSlot] = struct{}{}
			}
		}
	}
	out.RecentEnemyInteractorsBeforeCast = len(recentEnemies)

	recentAllies := make(map[int]struct{})
	if len(recentEnemies) > 0 {
		for _, damage := range tl.Damage {
			if damage.T < preStart || damage.T > castT {
				continue
			}
			if _, sameEnemy := recentEnemies[damage.VictimSlot]; sameEnemy {
				if attacker := keyAbilityPlayerForSlot(tl, damage.AttackerSlot); attacker != nil && attacker.PlayerSlot != target.PlayerSlot && attacker.Team == target.Team {
					recentAllies[attacker.PlayerSlot] = struct{}{}
				}
			}
			if _, sameEnemy := recentEnemies[damage.AttackerSlot]; sameEnemy {
				if victim := keyAbilityPlayerForSlot(tl, damage.VictimSlot); victim != nil && victim.PlayerSlot != target.PlayerSlot && victim.Team == target.Team {
					recentAllies[victim.PlayerSlot] = struct{}{}
				}
			}
		}
	}
	out.RecentAlliedTeammatesInteractingWithSameEnemies = len(recentAllies)

	followupEnd := castT + ChronosphereImmediateFollowupWindowSeconds
	targetVictims := make(map[int]struct{})
	for _, damage := range tl.Damage {
		if damage.T <= castT || damage.T > followupEnd || damage.AttackerSlot != target.PlayerSlot {
			continue
		}
		victim := keyAbilityPlayerForSlot(tl, damage.VictimSlot)
		if victim == nil || victim.Team == target.Team {
			continue
		}
		targetVictims[victim.PlayerSlot] = struct{}{}
		out.TargetHeroDamageInFollowup += int64(damage.Value)
		if out.SecondsToFirstTargetHeroDamageAfterCast == nil {
			delta := damage.T - castT
			out.SecondsToFirstTargetHeroDamageAfterCast = &delta
		}
	}
	out.TargetEnemyHeroesDamagedInFollowup = len(targetVictims)

	alliedContributors := make(map[int]struct{})
	if len(targetVictims) > 0 {
		for _, damage := range tl.Damage {
			if damage.T <= castT || damage.T > followupEnd {
				continue
			}
			if _, targetVictim := targetVictims[damage.VictimSlot]; !targetVictim {
				continue
			}
			attacker := keyAbilityPlayerForSlot(tl, damage.AttackerSlot)
			if attacker == nil || attacker.PlayerSlot == target.PlayerSlot || attacker.Team != target.Team {
				continue
			}
			alliedContributors[attacker.PlayerSlot] = struct{}{}
			out.AlliedHeroDamageToTargetVictimsInFollowup += int64(damage.Value)
		}
	}
	out.AlliedTeammatesDamagingTargetVictimsInFollowup = len(alliedContributors)

	return out, true
}
