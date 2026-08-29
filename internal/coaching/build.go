package coaching

import (
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
		FightIndex:                    source.FightIndex,
		FightObservedStartT:           source.FightObservedStartT,
		FightObservedEndT:             source.FightObservedEndT,
		WindowStartT:                  source.WindowStartT,
		WindowEndT:                    source.WindowEndT,
		WindowEndReason:               source.WindowEndReason,
		WindowDurationSeconds:         source.WindowDurationSeconds,
		TargetAliveAtFightEnd:         source.TargetAliveAtEnd,
		AlliedHeroesAliveAtFightEnd:   source.AlliedHeroesAliveAtEnd,
		AlliedDeaths:                  source.AlliedDeaths,
		EnemyDeaths:                   source.EnemyDeaths,
		EnemyDeathAdvantage:           source.EnemyDeathAdvantage,
		EnemyDeathsStillDeadAtEnd:     source.EnemyDeathsStillDeadAtWindowEnd,
		PushableTowerOptions:          pushable,
		RoshanKnowledgeState:          source.RoshanKnowledgeState,
		RoshanKnownAliveForDecision:   source.RoshanKnownAliveForDecision,
		TargetTeamConversionCount:     source.TargetTeamConversionCount,
		NoTargetTeamConversion:        source.NoTargetTeamConversion,
	}
	return evidence, source.FightObservedStartT, source.WindowEndT, true
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
