package coaching

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

// BuildMatchCoachingInput converts the deterministic detector candidate stream
// into the compact model intended for eventual AI consumption. The returned
// value contains no MatchTimeline reference and no detector assessments.
//
// Candidate evidence is adapted explicitly by type. Unknown or malformed
// candidate shapes fail closed and are omitted rather than forwarding a wider
// object that could accidentally expand the coaching boundary.
func BuildMatchCoachingInput(tl *timeline.MatchTimeline) MatchCoachingInput {
	out := MatchCoachingInput{Moments: []CoachingMoment{}}
	if tl == nil {
		return out
	}

	out.MatchID = tl.MatchID
	out.Hero = targetHero(tl)

	deathAnalysis := detector.AnalyzeDeaths(tl)
	for _, candidate := range deathAnalysis.Candidates {
		evidence, ok := deathCandidateEvidence(candidate)
		if !ok {
			continue
		}
		out.Moments = append(out.Moments, CoachingMoment{
			Type:       candidate.Type,
			StartT:     candidate.T,
			EndT:       candidate.T,
			Confidence: candidate.Confidence,
			Evidence:   evidence,
		})
	}

	fightAnalysis := detector.AnalyzeFights(tl)
	for _, candidate := range fightAnalysis.Candidates {
		evidence, ok := fightCandidateEvidence(candidate)
		if !ok {
			continue
		}
		out.Moments = append(out.Moments, CoachingMoment{
			Type:       candidate.Type,
			StartT:     candidate.T,
			EndT:       candidate.T,
			Confidence: candidate.Confidence,
			Evidence:   evidence,
		})
	}

	postWaveAnalysis := detector.AnalyzePostWaves(tl)
	for _, candidate := range postWaveAnalysis.Candidates {
		evidence, startT, endT, ok := postWaveCandidateEvidence(candidate)
		if !ok {
			continue
		}
		out.Moments = append(out.Moments, CoachingMoment{
			Type:       candidate.Type,
			StartT:     startT,
			EndT:       endT,
			Confidence: candidate.Confidence,
			Evidence:   evidence,
		})
	}

	objectiveAnalysis := detector.AnalyzeObjectives(tl)
	for _, candidate := range objectiveAnalysis.Candidates {
		evidence, startT, endT, ok := objectiveCandidateEvidence(candidate)
		if !ok {
			continue
		}
		out.Moments = append(out.Moments, CoachingMoment{
			Type:       candidate.Type,
			StartT:     startT,
			EndT:       endT,
			Confidence: candidate.Confidence,
			Evidence:   evidence,
		})
	}

	keyAbilityAnalysis := detector.AnalyzeKeyAbilities(tl)
	for _, candidate := range keyAbilityAnalysis.Candidates {
		evidence, t, ok := keyAbilityCandidateEvidence(tl, candidate)
		if !ok {
			continue
		}
		out.Moments = append(out.Moments, CoachingMoment{
			Type:       candidate.Type,
			StartT:     t,
			EndT:       t,
			Confidence: candidate.Confidence,
			Evidence:   evidence,
		})
	}

	sort.SliceStable(out.Moments, func(i, j int) bool {
		if out.Moments[i].StartT == out.Moments[j].StartT {
			return out.Moments[i].Type < out.Moments[j].Type
		}
		return out.Moments[i].StartT < out.Moments[j].StartT
	})
	return out
}

func deathCandidateEvidence(candidate detector.Candidate) (any, bool) {
	switch candidate.Type {
	case detector.TypeIsolatedDeathCandidate:
		if candidate.Isolation == nil {
			return nil, false
		}
		return *candidate.Isolation, true
	case detector.TypePreFightDeathCandidate:
		if candidate.PreFight == nil {
			return nil, false
		}
		return *candidate.PreFight, true
	default:
		return nil, false
	}
}

func fightCandidateEvidence(candidate detector.FightCandidate) (any, bool) {
	switch candidate.Type {
	case detector.TypeBadFightJoinCandidate:
		if candidate.BadFightJoin == nil {
			return nil, false
		}
		return *candidate.BadFightJoin, true
	case detector.TypeMissedFightCandidate:
		if candidate.MissedFight == nil {
			return nil, false
		}
		return *candidate.MissedFight, true
	default:
		return nil, false
	}
}

func postWaveCandidateEvidence(candidate detector.PostWaveCandidate) (PostWaveOverstayReviewEvidence, float64, float64, bool) {
	if candidate.Type != detector.TypePostWaveOverstayCandidate || candidate.PostWave == nil {
		return PostWaveOverstayReviewEvidence{}, 0, 0, false
	}
	source := candidate.PostWave
	if source.WaveID == "" || !validObjectiveLane(source.Lane) ||
		!finite(source.LastDepletionT) || !finite(source.ExposureEndT) ||
		source.ExposureEndT <= source.LastDepletionT ||
		!finite(source.PostClearDurationSeconds) || source.PostClearDurationSeconds <= 0 ||
		source.PostClearLaneProgressDeltaWorld == nil ||
		!finite(*source.PostClearLaneProgressDeltaWorld) || *source.PostClearLaneProgressDeltaWorld <= 0 ||
		!source.TargetAvailableAtClear || !source.TargetAliveAtClear ||
		source.TargetCombatStartedByLastDepletion || !source.TargetCombatStartedDuringPostClear ||
		source.SecondsFromClearToFirstInvolvement == nil ||
		!finite(*source.SecondsFromClearToFirstInvolvement) || *source.SecondsFromClearToFirstInvolvement <= 0 {
		return PostWaveOverstayReviewEvidence{}, 0, 0, false
	}
	if source.TargetFirstInvolvementT == nil || !finite(*source.TargetFirstInvolvementT) ||
		*source.TargetFirstInvolvementT <= source.LastDepletionT || *source.TargetFirstInvolvementT > source.ExposureEndT {
		return PostWaveOverstayReviewEvidence{}, 0, 0, false
	}
	if source.NextTargetDeathT != nil && (!finite(*source.NextTargetDeathT) || *source.NextTargetDeathT < source.LastDepletionT) {
		return PostWaveOverstayReviewEvidence{}, 0, 0, false
	}

	evidence := PostWaveOverstayReviewEvidence{
		WaveID:                             source.WaveID,
		Lane:                               source.Lane,
		LastDepletionT:                     source.LastDepletionT,
		ExposureEndT:                       source.ExposureEndT,
		PostClearDurationSeconds:           source.PostClearDurationSeconds,
		PostClearLaneProgressDeltaWorld:    *source.PostClearLaneProgressDeltaWorld,
		TargetCombatStartedDuringPostClear: source.TargetCombatStartedDuringPostClear,
		SecondsFromClearToFirstInvolvement: *source.SecondsFromClearToFirstInvolvement,
		TargetFirstInvolvementSource:       source.TargetFirstInvolvementSource,
		NextWaveTakingObserved:             source.NextWaveTakingObserved,
		NextTargetDeathT:                   copyFloat64(source.NextTargetDeathT),
	}
	return evidence, source.LastDepletionT, source.ExposureEndT, true
}

func objectiveCandidateEvidence(candidate detector.ObjectiveCandidate) (PostFightConversionReviewEvidence, float64, float64, bool) {
	if candidate.Type != detector.TypePostFightConversionReviewCandidate || candidate.Objective == nil {
		return PostFightConversionReviewEvidence{}, 0, 0, false
	}
	source := candidate.Objective
	if source.FightObservedEndT < source.FightObservedStartT || source.WindowStartT < source.FightObservedEndT || source.WindowEndT < source.WindowStartT {
		return PostFightConversionReviewEvidence{}, 0, 0, false
	}

	pushable := make([]ObjectiveReviewTowerOption, 0, len(source.EnemyPushableTowerOptions))
	for _, option := range source.EnemyPushableTowerOptions {
		if !validObjectiveLane(option.Lane) || option.Tier < 1 || option.Tier > 3 {
			return PostFightConversionReviewEvidence{}, 0, 0, false
		}
		pushable = append(pushable, ObjectiveReviewTowerOption{Lane: option.Lane, Tier: option.Tier})
	}

	evidence := PostFightConversionReviewEvidence{
		FightIndex:                  source.FightIndex,
		FightObservedStartT:         source.FightObservedStartT,
		FightObservedEndT:           source.FightObservedEndT,
		WindowStartT:                source.WindowStartT,
		WindowEndT:                  source.WindowEndT,
		WindowEndReason:             source.WindowEndReason,
		WindowDurationSeconds:       source.WindowDurationSeconds,
		TargetAliveAtFightEnd:       source.TargetAliveAtEnd,
		AlliedHeroesAliveAtFightEnd: source.AlliedHeroesAliveAtEnd,
		AlliedDeaths:                source.AlliedDeaths,
		EnemyDeaths:                 source.EnemyDeaths,
		EnemyDeathAdvantage:         source.EnemyDeathAdvantage,
		EnemyDeathsStillDeadAtEnd:   source.EnemyDeathsStillDeadAtWindowEnd,
		PushableTowerOptions:        pushable,
		RoshanKnowledgeState:        source.RoshanKnowledgeState,
		RoshanKnownAliveForDecision: source.RoshanKnownAliveForDecision,
		TargetTeamConversionCount:   source.TargetTeamConversionCount,
		NoTargetTeamConversion:      source.NoTargetTeamConversion,
	}
	return evidence, source.FightObservedStartT, source.WindowEndT, true
}

func keyAbilityCandidateEvidence(tl *timeline.MatchTimeline, candidate detector.KeyAbilityCandidate) (any, float64, bool) {
	switch candidate.Type {
	case detector.TypeKeyAbilityUseReviewCandidate:
		if candidate.KeyAbility == nil {
			return nil, 0, false
		}
		source := candidate.KeyAbility
		if source.Ability == "" || !finite(source.CastT) || source.PreCastWindowSeconds <= 0 ||
			!finite(source.PreCastWindowSeconds) || source.OutcomeWindowSeconds <= 0 || !finite(source.OutcomeWindowSeconds) ||
			source.AlliedTeammatesAliveAtCast < 0 || source.EnemyDeathsAfterCast < 0 || source.AlliedDeathsAfterCast < 0 {
			return nil, 0, false
		}
		if source.TargetHPPctAtCast != nil && (!finite(*source.TargetHPPctAtCast) || *source.TargetHPPctAtCast < 0) {
			return nil, 0, false
		}
		if source.TargetDeathT != nil && (!finite(*source.TargetDeathT) || *source.TargetDeathT <= source.CastT || *source.TargetDeathT > source.CastT+source.OutcomeWindowSeconds) {
			return nil, 0, false
		}
		evidence := KeyAbilityReviewEvidence{
			Ability:                        source.Ability,
			CastT:                          source.CastT,
			TargetSampleAvailable:          source.TargetSampleAvailable,
			TargetAliveAtCast:              source.TargetAliveAtCast,
			TargetHPAtCast:                 source.TargetHPAtCast,
			TargetMaxHPAtCast:              source.TargetMaxHPAtCast,
			TargetHPPctAtCast:              copyFloat64(source.TargetHPPctAtCast),
			AlliedTeammatesAliveAtCast:     source.AlliedTeammatesAliveAtCast,
			PreCastWindowSeconds:           source.PreCastWindowSeconds,
			TargetDamageDealtBeforeCast:    source.TargetDamageDealtBeforeCast,
			TargetDamageReceivedBeforeCast: source.TargetDamageReceivedBeforeCast,
			OutcomeWindowSeconds:           source.OutcomeWindowSeconds,
			TargetDamageDealtAfterCast:     source.TargetDamageDealtAfterCast,
			TargetDamageReceivedAfterCast:  source.TargetDamageReceivedAfterCast,
			EnemyDeathsAfterCast:           source.EnemyDeathsAfterCast,
			AlliedDeathsAfterCast:          source.AlliedDeathsAfterCast,
			TargetDeathT:                   copyFloat64(source.TargetDeathT),
			TargetDeathInflictor:           source.TargetDeathInflictor,
		}
		if source.Ability != "faceless_void_chronosphere" {
			return evidence, source.CastT, true
		}

		context, ok := detector.ChronosphereCombatContextAt(tl, source.CastT)
		if !ok || !finite(context.FollowupWindowSeconds) || context.FollowupWindowSeconds <= 0 ||
			context.FollowupWindowEqualsSpellDuration || context.CaughtHeroesConfirmedFromReplay || context.CastPlacementConfirmedFromReplay ||
			context.RecentEnemyInteractorsBeforeCast < 0 || context.RecentAlliedTeammatesInteractingWithSameEnemies < 0 ||
			context.TargetEnemyHeroesDamagedInFollowup < 0 || context.TargetHeroDamageInFollowup < 0 ||
			context.AlliedTeammatesDamagingTargetVictimsInFollowup < 0 || context.AlliedHeroDamageToTargetVictimsInFollowup < 0 {
			return nil, 0, false
		}
		if context.SecondsToFirstTargetHeroDamageAfterCast != nil &&
			(!finite(*context.SecondsToFirstTargetHeroDamageAfterCast) || *context.SecondsToFirstTargetHeroDamageAfterCast <= 0 || *context.SecondsToFirstTargetHeroDamageAfterCast > context.FollowupWindowSeconds) {
			return nil, 0, false
		}
		if context.TargetEnemyHeroesDamagedInFollowup == 0 {
			if context.TargetHeroDamageInFollowup != 0 || context.SecondsToFirstTargetHeroDamageAfterCast != nil ||
				context.AlliedTeammatesDamagingTargetVictimsInFollowup != 0 || context.AlliedHeroDamageToTargetVictimsInFollowup != 0 {
				return nil, 0, false
			}
		} else if context.TargetHeroDamageInFollowup <= 0 || context.SecondsToFirstTargetHeroDamageAfterCast == nil {
			return nil, 0, false
		}
		if (context.AlliedTeammatesDamagingTargetVictimsInFollowup == 0) != (context.AlliedHeroDamageToTargetVictimsInFollowup == 0) {
			return nil, 0, false
		}

		return ChronosphereKeyAbilityReviewEvidence{
			KeyAbilityReviewEvidence: evidence,
			ChronosphereFollowup: ChronosphereFollowupReviewEvidence{
				FollowupWindowSeconds:                           context.FollowupWindowSeconds,
				FollowupWindowEqualsSpellDuration:               context.FollowupWindowEqualsSpellDuration,
				CaughtHeroesConfirmedFromReplay:                 context.CaughtHeroesConfirmedFromReplay,
				CastPlacementConfirmedFromReplay:                context.CastPlacementConfirmedFromReplay,
				RecentEnemyInteractorsBeforeCast:                context.RecentEnemyInteractorsBeforeCast,
				RecentAlliedTeammatesInteractingWithSameEnemies: context.RecentAlliedTeammatesInteractingWithSameEnemies,
				TargetEnemyHeroesDamagedInFollowup:              context.TargetEnemyHeroesDamagedInFollowup,
				TargetHeroDamageInFollowup:                      context.TargetHeroDamageInFollowup,
				AlliedTeammatesDamagingTargetVictimsInFollowup:  context.AlliedTeammatesDamagingTargetVictimsInFollowup,
				AlliedHeroDamageToTargetVictimsInFollowup:       context.AlliedHeroDamageToTargetVictimsInFollowup,
				SecondsToFirstTargetHeroDamageAfterCast:          copyFloat64(context.SecondsToFirstTargetHeroDamageAfterCast),
			},
		}, source.CastT, true

	case detector.TypeActiveDamageReflectInteractionCandidate:
		if candidate.ActiveDamageReflect == nil {
			return nil, 0, false
		}
		source := candidate.ActiveDamageReflect
		if source.Ability == "" || source.Item == "" || !finite(source.CastT) || !finite(source.ItemUseT) ||
			source.ItemUseT > source.CastT || !finite(source.SecondsFromItemUseToCast) || source.SecondsFromItemUseToCast < 0 ||
			source.PlayerKnowledgeStatus != detector.PlayerKnowledgeNotConfirmedFromReplay ||
			!finite(source.OutcomeWindowSeconds) || source.OutcomeWindowSeconds <= 0 || source.ReflectedDamageAfterCast <= 0 {
			return nil, 0, false
		}
		if source.FirstReflectedDamageT == nil || !finite(*source.FirstReflectedDamageT) || *source.FirstReflectedDamageT <= source.CastT || *source.FirstReflectedDamageT > source.CastT+source.OutcomeWindowSeconds {
			return nil, 0, false
		}
		if source.TargetDeathToReflect {
			if source.TargetDeathT == nil || !finite(*source.TargetDeathT) || *source.TargetDeathT <= source.CastT || *source.TargetDeathT > source.CastT+source.OutcomeWindowSeconds {
				return nil, 0, false
			}
		} else if source.TargetDeathT != nil {
			return nil, 0, false
		}
		evidence := ActiveDamageReflectReviewEvidence{
			Ability:                  source.Ability,
			CastT:                    source.CastT,
			Item:                     source.Item,
			ItemUseT:                 source.ItemUseT,
			SecondsFromItemUseToCast: source.SecondsFromItemUseToCast,
			PlayerKnowledgeStatus:    source.PlayerKnowledgeStatus,
			OutcomeWindowSeconds:     source.OutcomeWindowSeconds,
			ReflectedDamageAfterCast: source.ReflectedDamageAfterCast,
			FirstReflectedDamageT:    copyFloat64(source.FirstReflectedDamageT),
			TargetDeathToReflect:     source.TargetDeathToReflect,
			TargetDeathT:             copyFloat64(source.TargetDeathT),
		}
		return evidence, source.CastT, true
	default:
		return nil, 0, false
	}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func copyFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func validObjectiveLane(lane string) bool {
	switch lane {
	case "top", "mid", "bottom":
		return true
	default:
		return false
	}
}

func targetHero(tl *timeline.MatchTimeline) string {
	if tl == nil {
		return ""
	}
	if player := tl.Players[strconv.Itoa(tl.TargetPlayerSlot)]; player != nil {
		return compactHeroName(player)
	}
	for _, player := range tl.Players {
		if player != nil && player.PlayerSlot == tl.TargetPlayerSlot {
			return compactHeroName(player)
		}
	}
	return ""
}

func compactHeroName(player *timeline.PlayerTimeline) string {
	if player == nil {
		return ""
	}
	if player.HeroName != "" {
		return strings.TrimPrefix(player.HeroName, "npc_dota_hero_")
	}
	if player.HeroClass != "" {
		return strings.ToLower(strings.TrimPrefix(player.HeroClass, "CDOTA_Unit_Hero_"))
	}
	return ""
}
