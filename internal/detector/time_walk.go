package detector

import "github.com/ap0calypse644/git_gud/internal/timeline"

const (
	TypeTimeWalkDamageRecoveryReviewCandidate = "time_walk_damage_recovery_review_candidate"

	TimeWalkPreDamageWindowSeconds       = 2.0
	TimeWalkPostCastSampleWindowSeconds  = 2.0
	TimeWalkOutcomeWindowSeconds         = 3.0
	TimeWalkMinRecentDamagePctMaxHP      = 0.20
)

// TimeWalkDamageRecoveryEvidence reviews Time Walk as a damage-backtrack
// decision. Candidate selection uses only pre-cast replay facts. Samples after
// the cast, later damage, and death are retrospective outcomes and must not be
// promoted into decision-time knowledge downstream.
type TimeWalkDamageRecoveryEvidence struct {
	Hero                         string   `json:"hero"`
	Ability                      string   `json:"ability"`
	CastT                        float64  `json:"cast_t"`
	TargetSampleAvailable        bool     `json:"target_sample_available"`
	TargetAliveAtCast            bool     `json:"target_alive_at_cast"`
	TargetSampleT                float64  `json:"target_sample_t"`
	TargetSampleAgeSeconds       float64  `json:"target_sample_age_seconds"`
	TargetHPAtCastSample         int32    `json:"target_hp_at_cast_sample"`
	TargetMaxHPAtCastSample      int32    `json:"target_max_hp_at_cast_sample"`
	TargetHPPctAtCastSample      *float64 `json:"target_hp_pct_at_cast_sample,omitempty"`
	PreDamageWindowSeconds       float64  `json:"pre_damage_window_seconds"`
	IncomingDamageBeforeCast     int64    `json:"incoming_damage_before_cast"`
	IncomingDamagePctMaxHP       float64  `json:"incoming_damage_pct_max_hp"`
	PostCastSampleWindowSeconds  float64  `json:"post_cast_sample_window_seconds"`
	PostCastSampleAvailable      bool     `json:"post_cast_sample_available"`
	PostCastSampleT              *float64 `json:"post_cast_sample_t,omitempty"`
	PostCastSampleDelaySeconds   *float64 `json:"post_cast_sample_delay_seconds,omitempty"`
	TargetHPAtPostCastSample     int32    `json:"target_hp_at_post_cast_sample,omitempty"`
	TargetMaxHPAtPostCastSample  int32    `json:"target_max_hp_at_post_cast_sample,omitempty"`
	TargetHPPctAtPostCastSample  *float64 `json:"target_hp_pct_at_post_cast_sample,omitempty"`
	OutcomeWindowSeconds         float64  `json:"outcome_window_seconds"`
	IncomingDamageAfterCast      int64    `json:"incoming_damage_after_cast"`
	TargetDeathT                 *float64 `json:"target_death_t,omitempty"`
}

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
			Type:                   TypeTimeWalkDamageRecoveryReviewCandidate,
			T:                      cast.T,
			Confidence:             ConfidenceLow,
			TimeWalkDamageRecovery: &copied,
		})
	}
}

func buildTimeWalkDamageRecoveryEvidence(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, cast timeline.AbilityEvent) (TimeWalkDamageRecoveryEvidence, bool) {
	preSample, ok := keyAbilityLatestSampleAtOrBefore(target.Samples, cast.T)
	if !ok || !preSample.Alive || preSample.MaxHP <= 0 || preSample.HP < 0 {
		return TimeWalkDamageRecoveryEvidence{}, false
	}

	incomingBefore := timeWalkIncomingDamage(tl, target.PlayerSlot, cast.T-TimeWalkPreDamageWindowSeconds, cast.T)
	incomingPct := float64(incomingBefore) / float64(preSample.MaxHP)
	if incomingPct < TimeWalkMinRecentDamagePctMaxHP {
		return TimeWalkDamageRecoveryEvidence{}, false
	}

	hpPct := float64(preSample.HP) / float64(preSample.MaxHP)
	out := TimeWalkDamageRecoveryEvidence{
		Hero:                        target.HeroName,
		Ability:                     cast.Ability,
		CastT:                       cast.T,
		TargetSampleAvailable:       true,
		TargetAliveAtCast:           preSample.Alive,
		TargetSampleT:               preSample.T,
		TargetSampleAgeSeconds:      cast.T - preSample.T,
		TargetHPAtCastSample:        preSample.HP,
		TargetMaxHPAtCastSample:     preSample.MaxHP,
		TargetHPPctAtCastSample:     &hpPct,
		PreDamageWindowSeconds:      TimeWalkPreDamageWindowSeconds,
		IncomingDamageBeforeCast:    incomingBefore,
		IncomingDamagePctMaxHP:      incomingPct,
		PostCastSampleWindowSeconds: TimeWalkPostCastSampleWindowSeconds,
		OutcomeWindowSeconds:        TimeWalkOutcomeWindowSeconds,
		IncomingDamageAfterCast:     timeWalkIncomingDamage(tl, target.PlayerSlot, cast.T, cast.T+TimeWalkOutcomeWindowSeconds),
	}

	if postSample, ok := timeWalkFirstSampleAfterWithin(target.Samples, cast.T, cast.T+TimeWalkPostCastSampleWindowSeconds); ok {
		out.PostCastSampleAvailable = true
		t := postSample.T
		delay := postSample.T - cast.T
		out.PostCastSampleT = &t
		out.PostCastSampleDelaySeconds = &delay
		out.TargetHPAtPostCastSample = postSample.HP
		out.TargetMaxHPAtPostCastSample = postSample.MaxHP
		if postSample.MaxHP > 0 {
			pct := float64(postSample.HP) / float64(postSample.MaxHP)
			out.TargetHPPctAtPostCastSample = &pct
		}
	}

	for _, death := range tl.Deaths {
		if death.VictimSlot == nil || *death.VictimSlot != target.PlayerSlot || death.T <= cast.T || death.T > cast.T+TimeWalkOutcomeWindowSeconds {
			continue
		}
		t := death.T
		out.TargetDeathT = &t
		break
	}
	return out, true
}

func timeWalkIncomingDamage(tl *timeline.MatchTimeline, targetSlot int, startT, endT float64) int64 {
	if tl == nil {
		return 0
	}
	var total int64
	for _, damage := range tl.Damage {
		if damage.VictimSlot != targetSlot || damage.T < startT || damage.T > endT || damage.Value <= 0 {
			continue
		}
		total += int64(damage.Value)
	}
	return total
}

func timeWalkFirstSampleAfterWithin(samples []timeline.HeroSample, castT, endT float64) (timeline.HeroSample, bool) {
	var best timeline.HeroSample
	found := false
	for _, sample := range samples {
		if sample.T <= castT || sample.T > endT {
			continue
		}
		if !found || sample.T < best.T {
			best = sample
			found = true
		}
	}
	return best, found
}
