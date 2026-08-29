package detector

import (
	"sort"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const TypeObjectiveMissCandidate = "objective_miss_candidate"

// ObjectiveAnalysis keeps an assessment for every post-fight objective context
// so calibration can inspect both emitted candidates and suppressed cases.
// Candidates are low-confidence review targets only; they are not a claim that
// Roshan should definitely have been taken.
type ObjectiveAnalysis struct {
	MatchID     int64                 `json:"match_id"`
	Assessments []ObjectiveAssessment `json:"assessments"`
	Candidates  []ObjectiveCandidate  `json:"candidates"`
}

type ObjectiveAssessment struct {
	FightIndex int                   `json:"fight_index"`
	T          float64               `json:"t"`
	Candidate  bool                  `json:"candidate"`
	Evidence   ObjectiveMissEvidence `json:"evidence"`
}

type ObjectiveCandidate struct {
	Type       string                 `json:"type"`
	T          float64                `json:"t"`
	Confidence string                 `json:"confidence"`
	Objective  *ObjectiveMissEvidence `json:"objective,omitempty"`
}

// ObjectiveMissEvidence contains only decision-safe state plus retrospective
// conversion outcomes. Exact hidden Roshan respawn truth is deliberately not
// copied from timeline.RoshanPostFightState; only its causal knowledge state is
// retained here.
type ObjectiveMissEvidence struct {
	FightIndex            int     `json:"fight_index"`
	FightObservedStartT   float64 `json:"fight_observed_start_t"`
	FightObservedEndT     float64 `json:"fight_observed_end_t"`
	WindowEndT            float64 `json:"window_end_t"`
	WindowEndReason       string  `json:"window_end_reason"`
	WindowDurationSeconds float64 `json:"window_duration_seconds"`

	TargetInvolved          bool `json:"target_involved"`
	TargetEndSampleAvailable bool `json:"target_end_sample_available"`
	TargetAliveAtEnd         bool `json:"target_alive_at_end"`
	AlliedEndSamplesAvailable int `json:"allied_end_samples_available"`
	AlliedHeroesAliveAtEnd    int `json:"allied_heroes_alive_at_end"`
	AlliedDeaths              int `json:"allied_deaths"`
	EnemyDeaths               int `json:"enemy_deaths"`
	EnemyDeathAdvantage       int `json:"enemy_death_advantage"`

	EnemyTier1sDestroyedAtEnd []string `json:"enemy_tier1s_destroyed_at_end"`
	EnemyMapOpened            bool     `json:"enemy_map_opened"`

	RoshanKnowledgeState        string `json:"roshan_knowledge_state,omitempty"`
	RoshanKnownAliveForDecision bool   `json:"roshan_known_alive_for_decision"`

	TargetTeamConversionCount int  `json:"target_team_conversion_count"`
	NoTargetTeamConversion    bool `json:"no_target_team_conversion"`
}

// AnalyzeObjectives emits a deliberately conservative low-confidence review
// candidate. The gate avoids arbitrary seconds/distance/level thresholds:
//   - the target participated and survived;
//   - the team won a clean fight (enemy death(s), no allied deaths);
//   - complete five-player allied end-state sampling says all allies were alive;
//   - at least one enemy tier-one tower was already down, a conservative
//     map-state signal that suppresses obvious early-game Roshan noise;
//   - Roshan was causally knowable as alive (never a hidden random respawn);
//   - a real non-overlapping post-fight interval existed; and
//   - the target team converted neither Roshan nor a building in that interval.
//
// This remains a review candidate rather than a strategic verdict because the
// detector does not yet model Roshan-pit distance, team damage capability, or
// travel/path safety.
func AnalyzeObjectives(tl *timeline.MatchTimeline) ObjectiveAnalysis {
	out := ObjectiveAnalysis{Assessments: []ObjectiveAssessment{}, Candidates: []ObjectiveCandidate{}}
	if tl == nil {
		return out
	}
	out.MatchID = tl.MatchID

	contexts := tl.TargetPostFightObjectives.Contexts
	if len(contexts) == 0 && tl.TargetPostFightObjectives.Available {
		contexts = timeline.DerivePostFightObjectiveTimeline(tl).Contexts
	}

	for _, ctx := range contexts {
		assessment := assessObjectiveMiss(ctx)
		out.Assessments = append(out.Assessments, assessment)
		if assessment.Candidate {
			evidence := assessment.Evidence
			out.Candidates = append(out.Candidates, ObjectiveCandidate{
				Type:       TypeObjectiveMissCandidate,
				T:          ctx.FightObservedEndT,
				Confidence: ConfidenceLow,
				Objective:  &evidence,
			})
		}
	}

	sort.SliceStable(out.Assessments, func(i, j int) bool {
		if out.Assessments[i].T == out.Assessments[j].T {
			return out.Assessments[i].FightIndex < out.Assessments[j].FightIndex
		}
		return out.Assessments[i].T < out.Assessments[j].T
	})
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].T == out.Candidates[j].T {
			return out.Candidates[i].Type < out.Candidates[j].Type
		}
		return out.Candidates[i].T < out.Candidates[j].T
	})
	return out
}

func assessObjectiveMiss(ctx timeline.PostFightObjectiveContext) ObjectiveAssessment {
	destroyedT1s := make([]string, 0, 3)
	for _, state := range ctx.EnemyLaneStructuresAtEnd {
		if state.Tier1Destroyed {
			destroyedT1s = append(destroyedT1s, state.Lane)
		}
	}
	sort.Strings(destroyedT1s)

	evidence := ObjectiveMissEvidence{
		FightIndex:                    ctx.FightIndex,
		FightObservedStartT:           ctx.FightObservedStartT,
		FightObservedEndT:             ctx.FightObservedEndT,
		WindowEndT:                    ctx.WindowEndT,
		WindowEndReason:               ctx.WindowEndReason,
		WindowDurationSeconds:         ctx.WindowDurationSeconds,
		TargetInvolved:                ctx.TargetInvolved,
		TargetEndSampleAvailable:      ctx.TargetEndSampleAvailable,
		TargetAliveAtEnd:              ctx.TargetAliveAtEnd,
		AlliedEndSamplesAvailable:     ctx.AlliedEndSamplesAvailable,
		AlliedHeroesAliveAtEnd:        ctx.AlliedHeroesAliveAtEnd,
		AlliedDeaths:                  ctx.AlliedDeaths,
		EnemyDeaths:                   ctx.EnemyDeaths,
		EnemyDeathAdvantage:           ctx.EnemyDeathAdvantage,
		EnemyTier1sDestroyedAtEnd:     destroyedT1s,
		EnemyMapOpened:                len(destroyedT1s) > 0,
		RoshanKnowledgeState:          ctx.RoshanAtEnd.KnowledgeState,
		RoshanKnownAliveForDecision:   ctx.RoshanAtEnd.KnownAliveForDecision,
		TargetTeamConversionCount:     len(ctx.TargetTeamConversions),
		NoTargetTeamConversion:        len(ctx.TargetTeamConversions) == 0,
	}

	candidate := ctx.ObservedTimingAvailable &&
		evidence.WindowDurationSeconds > 0 &&
		evidence.WindowEndReason != "overlapping_fight_active" &&
		evidence.TargetInvolved &&
		evidence.TargetEndSampleAvailable &&
		evidence.TargetAliveAtEnd &&
		evidence.AlliedDeaths == 0 &&
		evidence.EnemyDeaths > 0 &&
		evidence.EnemyDeathAdvantage > 0 &&
		evidence.AlliedEndSamplesAvailable == 5 &&
		evidence.AlliedHeroesAliveAtEnd == 5 &&
		evidence.EnemyMapOpened &&
		evidence.RoshanKnownAliveForDecision &&
		evidence.NoTargetTeamConversion

	return ObjectiveAssessment{
		FightIndex: ctx.FightIndex,
		T:          ctx.FightObservedEndT,
		Candidate:  candidate,
		Evidence:   evidence,
	}
}
