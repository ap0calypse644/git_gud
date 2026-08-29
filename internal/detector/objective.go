package detector

import (
	"sort"
	"strconv"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const TypeObjectiveMissCandidate = "objective_miss_candidate"

// ObjectiveAnalysis keeps an assessment for every post-fight objective context
// so calibration can inspect both emitted candidates and suppressed cases.
// Candidates are low-confidence review targets only; they are not a claim that
// any particular objective definitely should have been taken.
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

// ObjectiveTowerOption is one lane-front tower that was causally known alive
// at fight end. Tier is the first still-alive tier in that lane after all
// lower tiers have observed destruction events.
type ObjectiveTowerOption struct {
	Lane string `json:"lane"`
	Tier int    `json:"tier"`
}

// ObjectiveMissEvidence contains only decision-safe state plus retrospective
// conversion/power-play outcomes. Exact hidden Roshan respawn truth is
// deliberately not copied from timeline.RoshanPostFightState; only its causal
// knowledge state is retained here.
type ObjectiveMissEvidence struct {
	FightIndex            int     `json:"fight_index"`
	FightObservedStartT   float64 `json:"fight_observed_start_t"`
	FightObservedEndT     float64 `json:"fight_observed_end_t"`
	WindowEndT            float64 `json:"window_end_t"`
	WindowEndReason       string  `json:"window_end_reason"`
	WindowDurationSeconds float64 `json:"window_duration_seconds"`

	TargetInvolved            bool `json:"target_involved"`
	TargetEndSampleAvailable  bool `json:"target_end_sample_available"`
	TargetAliveAtEnd          bool `json:"target_alive_at_end"`
	AlliedEndSamplesAvailable int  `json:"allied_end_samples_available"`
	AlliedHeroesAliveAtEnd    int  `json:"allied_heroes_alive_at_end"`
	AlliedDeaths              int  `json:"allied_deaths"`
	EnemyDeaths               int  `json:"enemy_deaths"`
	EnemyDeathAdvantage       int  `json:"enemy_death_advantage"`

	EnemyDeathWindowEndStateAvailable bool  `json:"enemy_death_window_end_state_available"`
	EnemyDeathVictimSlots             []int `json:"enemy_death_victim_slots"`
	EnemyDeathsStillDeadAtWindowEnd   int   `json:"enemy_deaths_still_dead_at_window_end"`

	EnemyTier1sDestroyedAtEnd []string               `json:"enemy_tier1s_destroyed_at_end"`
	EnemyMapOpened            bool                   `json:"enemy_map_opened"`
	EnemyFrontTowerOptions    []ObjectiveTowerOption `json:"enemy_front_tower_options"`

	RoshanKnowledgeState        string `json:"roshan_knowledge_state,omitempty"`
	RoshanKnownAliveForDecision bool   `json:"roshan_known_alive_for_decision"`
	KnownObjectiveOptionCount   int    `json:"known_objective_option_count"`
	KnownObjectiveOptions       bool   `json:"known_objective_options"`

	TargetTeamConversionCount int  `json:"target_team_conversion_count"`
	NoTargetTeamConversion    bool `json:"no_target_team_conversion"`
}

// AnalyzeObjectives emits a deliberately conservative low-confidence review
// candidate. The gate avoids arbitrary seconds/distance/level thresholds:
//   - the target participated and survived;
//   - the team won a clean fight (enemy death(s), no allied deaths);
//   - complete five-player allied end-state sampling says all allies were alive;
//   - at least one enemy tier-one tower was already down, a conservative
//     map-state signal that suppresses obvious early-game conversion noise;
//   - at least one objective was causally known to be available: either the
//     lane-front tower in an opened lane or Roshan known alive without hidden
//     random-respawn leakage;
//   - a real non-overlapping post-fight interval existed;
//   - at least one enemy killed in that fight was still dead at the end of the
//     clean interval, confirming a sustained power-play rather than a momentary
//     death-count advantage; and
//   - the target team converted neither Roshan nor a building in that interval.
//
// This remains a review candidate rather than a strategic verdict because the
// detector does not model path safety, objective damage capability, buyback
// intent, wave position, or team communication.
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
		assessment := assessObjectiveMiss(tl, ctx)
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

func assessObjectiveMiss(tl *timeline.MatchTimeline, ctx timeline.PostFightObjectiveContext) ObjectiveAssessment {
	destroyedT1s := make([]string, 0, 3)
	frontTowers := make([]ObjectiveTowerOption, 0, 3)
	for _, state := range ctx.EnemyLaneStructuresAtEnd {
		if state.Tier1Destroyed {
			destroyedT1s = append(destroyedT1s, state.Lane)
		}
		if option, ok := objectiveFrontTower(state); ok {
			frontTowers = append(frontTowers, option)
		}
	}
	sort.Strings(destroyedT1s)
	sort.SliceStable(frontTowers, func(i, j int) bool {
		if frontTowers[i].Lane == frontTowers[j].Lane {
			return frontTowers[i].Tier < frontTowers[j].Tier
		}
		return frontTowers[i].Lane < frontTowers[j].Lane
	})

	deathStateAvailable, deathSlots, stillDead := enemyDeathStateAtWindowEnd(tl, ctx)
	knownObjectiveOptionCount := len(frontTowers)
	if ctx.RoshanAtEnd.KnownAliveForDecision {
		knownObjectiveOptionCount++
	}
	evidence := ObjectiveMissEvidence{
		FightIndex:                         ctx.FightIndex,
		FightObservedStartT:                ctx.FightObservedStartT,
		FightObservedEndT:                  ctx.FightObservedEndT,
		WindowEndT:                         ctx.WindowEndT,
		WindowEndReason:                    ctx.WindowEndReason,
		WindowDurationSeconds:              ctx.WindowDurationSeconds,
		TargetInvolved:                     ctx.TargetInvolved,
		TargetEndSampleAvailable:           ctx.TargetEndSampleAvailable,
		TargetAliveAtEnd:                   ctx.TargetAliveAtEnd,
		AlliedEndSamplesAvailable:          ctx.AlliedEndSamplesAvailable,
		AlliedHeroesAliveAtEnd:             ctx.AlliedHeroesAliveAtEnd,
		AlliedDeaths:                       ctx.AlliedDeaths,
		EnemyDeaths:                        ctx.EnemyDeaths,
		EnemyDeathAdvantage:                ctx.EnemyDeathAdvantage,
		EnemyDeathWindowEndStateAvailable:  deathStateAvailable,
		EnemyDeathVictimSlots:              deathSlots,
		EnemyDeathsStillDeadAtWindowEnd:    stillDead,
		EnemyTier1sDestroyedAtEnd:          destroyedT1s,
		EnemyMapOpened:                     len(destroyedT1s) > 0,
		EnemyFrontTowerOptions:             frontTowers,
		RoshanKnowledgeState:               ctx.RoshanAtEnd.KnowledgeState,
		RoshanKnownAliveForDecision:        ctx.RoshanAtEnd.KnownAliveForDecision,
		KnownObjectiveOptionCount:          knownObjectiveOptionCount,
		KnownObjectiveOptions:              knownObjectiveOptionCount > 0,
		TargetTeamConversionCount:          len(ctx.TargetTeamConversions),
		NoTargetTeamConversion:             len(ctx.TargetTeamConversions) == 0,
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
		evidence.EnemyDeathWindowEndStateAvailable &&
		evidence.EnemyDeathsStillDeadAtWindowEnd > 0 &&
		evidence.EnemyMapOpened &&
		evidence.KnownObjectiveOptions &&
		evidence.NoTargetTeamConversion

	return ObjectiveAssessment{
		FightIndex: ctx.FightIndex,
		T:          ctx.FightObservedEndT,
		Candidate:  candidate,
		Evidence:   evidence,
	}
}

func objectiveFrontTower(state timeline.LaneStructureState) (ObjectiveTowerOption, bool) {
	switch {
	case state.Tier1KnownAlive:
		return ObjectiveTowerOption{Lane: state.Lane, Tier: 1}, true
	case state.Tier1Destroyed && state.Tier2KnownAlive:
		return ObjectiveTowerOption{Lane: state.Lane, Tier: 2}, true
	case state.Tier1Destroyed && state.Tier2Destroyed && state.Tier3KnownAlive:
		return ObjectiveTowerOption{Lane: state.Lane, Tier: 3}, true
	default:
		return ObjectiveTowerOption{}, false
	}
}

// enemyDeathStateAtWindowEnd uses only enemy deaths inside the attributed
// fight participant set and hero samples at or before the already-derived
// window boundary. If attribution/sample coverage is ambiguous, it fails
// closed instead of manufacturing a power-play signal.
func enemyDeathStateAtWindowEnd(tl *timeline.MatchTimeline, ctx timeline.PostFightObjectiveContext) (bool, []int, int) {
	if tl == nil || ctx.EnemyDeaths <= 0 || ctx.WindowEndT < ctx.FightObservedEndT {
		return false, []int{}, 0
	}

	participants := make(map[int]bool, len(ctx.Participants))
	for _, slot := range ctx.Participants {
		participants[slot] = true
	}
	enemyTeam := objectiveOpposingTeam(ctx.TargetTeam)
	if enemyTeam == 0 {
		return false, []int{}, 0
	}

	deathTBySlot := map[int]float64{}
	matchedDeathEvents := 0
	for _, event := range tl.Deaths {
		if event.T < ctx.FightObservedStartT || event.T > ctx.FightObservedEndT || event.VictimSlot == nil {
			continue
		}
		slot := *event.VictimSlot
		if !participants[slot] {
			continue
		}
		player := tl.Players[strconv.Itoa(slot)]
		if player == nil || player.Team != enemyTeam {
			continue
		}
		matchedDeathEvents++
		if previous, ok := deathTBySlot[slot]; !ok || event.T > previous {
			deathTBySlot[slot] = event.T
		}
	}
	if matchedDeathEvents != ctx.EnemyDeaths || len(deathTBySlot) == 0 {
		return false, []int{}, 0
	}

	slots := make([]int, 0, len(deathTBySlot))
	stillDead := 0
	for slot, deathT := range deathTBySlot {
		player := tl.Players[strconv.Itoa(slot)]
		sample, ok := objectiveHeroSampleAtOrBefore(player, ctx.WindowEndT)
		if !ok || sample.T < deathT {
			return false, []int{}, 0
		}
		slots = append(slots, slot)
		if !sample.Alive {
			stillDead++
		}
	}
	sort.Ints(slots)
	return true, slots, stillDead
}

func objectiveHeroSampleAtOrBefore(player *timeline.PlayerTimeline, t float64) (timeline.HeroSample, bool) {
	if player == nil {
		return timeline.HeroSample{}, false
	}
	for i := len(player.Samples) - 1; i >= 0; i-- {
		if player.Samples[i].T <= t {
			return player.Samples[i], true
		}
	}
	return timeline.HeroSample{}, false
}

func objectiveOpposingTeam(team int) int {
	switch team {
	case 2:
		return 3
	case 3:
		return 2
	default:
		return 0
	}
}
