package coaching

import (
	"sort"
	"strconv"

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

func targetHero(tl *timeline.MatchTimeline) string {
	if tl == nil {
		return ""
	}
	if player := tl.Players[strconv.Itoa(tl.TargetPlayerSlot)]; player != nil {
		return player.HeroName
	}
	for _, player := range tl.Players {
		if player != nil && player.PlayerSlot == tl.TargetPlayerSlot {
			return player.HeroName
		}
	}
	return ""
}
