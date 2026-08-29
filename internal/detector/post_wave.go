package detector

import (
	"sort"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const TypePostWaveOverstayCandidate = "post_wave_overstay_candidate"

// PostWaveAnalysis keeps an assessment for every M17 context so calibration can
// inspect both emitted candidates and false negatives. Candidates are review
// targets only; this detector does not emit coaching text or a greed verdict.
type PostWaveAnalysis struct {
	MatchID     int64                `json:"match_id"`
	Assessments []PostWaveAssessment `json:"assessments"`
	Candidates  []PostWaveCandidate  `json:"candidates"`
}

type PostWaveAssessment struct {
	WaveID    string                   `json:"wave_id"`
	T         float64                  `json:"t"`
	Candidate bool                     `json:"candidate"`
	Evidence  PostWaveOverstayEvidence `json:"evidence"`
}

type PostWaveCandidate struct {
	Type       string                    `json:"type"`
	T          float64                   `json:"t"`
	Confidence string                    `json:"confidence"`
	PostWave   *PostWaveOverstayEvidence `json:"post_wave,omitempty"`
}

// PostWaveOverstayEvidence separates the decision-time clear anchor from
// retrospective outcome/context. No exact enemy replay positions are present.
// The candidate gate intentionally uses only boolean/sign structure and does
// not introduce an arbitrary distance, duration, or death threshold.
type PostWaveOverstayEvidence struct {
	WaveID       string  `json:"wave_id"`
	Lane         string  `json:"lane"`
	LastDepletionT float64 `json:"last_depletion_t"`
	PrimaryEndT    float64 `json:"primary_end_t"`
	ExposureEndT   float64 `json:"exposure_end_t"`

	TargetAvailableAtClear bool `json:"target_available_at_clear"`
	TargetAliveAtClear     bool `json:"target_alive_at_clear"`

	PostClearDurationSeconds        float64  `json:"post_clear_duration_seconds"`
	PostClearLaneProgressDeltaWorld *float64 `json:"post_clear_lane_progress_delta_world,omitempty"`
	DepthAtClearWorld               *float64 `json:"depth_at_clear_world,omitempty"`
	FreshLivingAlliesAtClear        int      `json:"fresh_living_allies_at_clear"`
	MissingEnemiesAtClear           int      `json:"missing_enemies_at_clear"`

	TargetCombatStartedByLastDepletion  bool     `json:"target_combat_started_by_last_depletion"`
	TargetCombatStartedDuringPostClear  bool     `json:"target_combat_started_during_post_clear"`
	TargetFirstInvolvementT             *float64 `json:"target_first_involvement_t,omitempty"`
	SecondsFromClearToFirstInvolvement  *float64 `json:"seconds_from_clear_to_first_involvement,omitempty"`
	TargetFirstInvolvementSource        string   `json:"target_first_involvement_source,omitempty"`

	NextWaveTakingObserved bool     `json:"next_wave_taking_observed"`
	NextTargetDeathT       *float64 `json:"next_target_death_t,omitempty"`
	SecondsFromPrimaryEndToDeath *float64 `json:"seconds_from_primary_end_to_death,omitempty"`
}

// AnalyzePostWaves emits a deliberately low-confidence M17 review candidate
// when the farming objective is complete, the alive target continues deeper,
// and direct target combat begins inside that same raw post-clear exposure tail.
// Death, next-wave taking, absolute depth, and enemy-count changes are retained
// as evidence only and never decide candidacy.
func AnalyzePostWaves(tl *timeline.MatchTimeline) PostWaveAnalysis {
	out := PostWaveAnalysis{Assessments: []PostWaveAssessment{}, Candidates: []PostWaveCandidate{}}
	if tl == nil {
		return out
	}
	out.MatchID = tl.MatchID

	contexts := tl.TargetPostWaveOverstay.Contexts
	if len(contexts) == 0 && tl.TargetWaveDanger.Available {
		contexts = timeline.DeriveTargetPostWaveOverstay(tl).Contexts
	}

	for _, ctx := range contexts {
		assessment := assessPostWaveOverstay(ctx)
		out.Assessments = append(out.Assessments, assessment)
		if assessment.Candidate {
			evidence := assessment.Evidence
			out.Candidates = append(out.Candidates, PostWaveCandidate{
				Type:       TypePostWaveOverstayCandidate,
				T:          ctx.LastDepletionT,
				Confidence: ConfidenceLow,
				PostWave:   &evidence,
			})
		}
	}

	sort.SliceStable(out.Assessments, func(i, j int) bool {
		if out.Assessments[i].T == out.Assessments[j].T {
			return out.Assessments[i].WaveID < out.Assessments[j].WaveID
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

func assessPostWaveOverstay(ctx timeline.TargetPostWaveOverstayContext) PostWaveAssessment {
	evidence := PostWaveOverstayEvidence{
		WaveID:                              ctx.WaveID,
		Lane:                                ctx.Lane,
		LastDepletionT:                      ctx.LastDepletionT,
		PrimaryEndT:                         ctx.PrimaryEndT,
		ExposureEndT:                        ctx.ExposureEndT,
		TargetAvailableAtClear:              ctx.LastDepletionState.TargetAvailable,
		TargetAliveAtClear:                  ctx.LastDepletionState.TargetAlive,
		PostClearDurationSeconds:            ctx.PostClear.DurationSeconds,
		PostClearLaneProgressDeltaWorld:     copyFloat64Ptr(ctx.PostClear.LaneProgressDeltaWorld),
		DepthAtClearWorld:                   copyFloat64Ptr(ctx.LastDepletionState.ForwardOfFriendlyReferenceWorld),
		FreshLivingAlliesAtClear:            ctx.LastDepletionState.FreshLivingAllies,
		MissingEnemiesAtClear:               ctx.LastDepletionState.MissingEnemies,
		TargetCombatStartedByLastDepletion:  ctx.CombatContext.TargetCombatStartedByLastDepletion,
		TargetCombatStartedDuringPostClear:  ctx.CombatContext.TargetCombatStartedDuringPostClear,
		TargetFirstInvolvementT:             copyFloat64Ptr(ctx.CombatContext.TargetFirstInvolvementT),
		SecondsFromClearToFirstInvolvement:  copyFloat64Ptr(ctx.CombatContext.SecondsFromLastDepletionToFirstInvolvement),
		TargetFirstInvolvementSource:        ctx.CombatContext.TargetFirstInvolvementSource,
		NextWaveTakingObserved:              ctx.NextCohort.TargetTakingObserved,
		NextTargetDeathT:                    copyFloat64Ptr(ctx.Outcome.NextTargetDeathT),
		SecondsFromPrimaryEndToDeath:        copyFloat64Ptr(ctx.Outcome.SecondsFromPrimaryEndToDeath),
	}

	candidate := evidence.TargetAvailableAtClear &&
		evidence.TargetAliveAtClear &&
		evidence.PostClearDurationSeconds > 0 &&
		evidence.PostClearLaneProgressDeltaWorld != nil &&
		*evidence.PostClearLaneProgressDeltaWorld > 0 &&
		!evidence.TargetCombatStartedByLastDepletion &&
		evidence.TargetCombatStartedDuringPostClear &&
		evidence.TargetFirstInvolvementT != nil &&
		evidence.SecondsFromClearToFirstInvolvement != nil &&
		*evidence.SecondsFromClearToFirstInvolvement > 0

	return PostWaveAssessment{
		WaveID:    ctx.WaveID,
		T:         ctx.LastDepletionT,
		Candidate: candidate,
		Evidence:  evidence,
	}
}
