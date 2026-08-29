package detector

import (
	"math"
	"sort"
	"strconv"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const (
	TypeObjectiveMissCandidate = "post_fight_conversion_review_candidate"

	// Source 2 timeline coordinates use 128 world units per cell.
	objectiveWorldScale = 128.0

	// Current Dota backdoor mechanics: T2 has an independent 900-world-unit
	// lane-creep detection radius; base buildings share a 4000-world-unit
	// Ancient-centered detection radius; protection remains disabled for 15s
	// after qualifying lane creeps leave/die. These are game mechanics, not
	// calibration thresholds.
	objectiveT2BackdoorRadiusWorld   = 900.0
	objectiveBaseBackdoorRadiusWorld = 4000.0
	objectiveBackdoorDisableSeconds  = 15.0
)

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
	WindowStartT          float64 `json:"window_start_t"`
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
	EnemyPushableTowerOptions []ObjectiveTowerOption `json:"enemy_pushable_tower_options"`

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
//   - at least one objective was causally supported: Roshan known alive without
//     hidden random-respawn leakage, an unprotected T1, or a protected T2/T3
//     with both conservative backdoor-disable creep evidence and an allied hero
//     observed inside that same mechanic-defined objective region during the
//     clean post-fight interval;
//   - a real non-overlapping post-fight interval existed;
//   - at least one enemy killed in that fight was still dead at the end of the
//     clean interval, confirming a sustained power-play rather than a momentary
//     death-count advantage; and
//   - the target team converted neither Roshan nor a building in that interval.
//
// This remains a review candidate rather than a strategic verdict because the
// detector still does not model path safety, Roshan/tower damage capability,
// buyback intent, enemy positioning beyond causal knowledge, or team
// communication.
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
	sortObjectiveTowerOptions(frontTowers)

	pushableTowers := objectivePushableTowerOptions(tl, ctx, frontTowers)
	deathStateAvailable, deathSlots, stillDead := enemyDeathStateAtWindowEnd(tl, ctx)
	knownObjectiveOptionCount := len(pushableTowers)
	if ctx.RoshanAtEnd.KnownAliveForDecision {
		knownObjectiveOptionCount++
	}
	evidence := ObjectiveMissEvidence{
		FightIndex:                        ctx.FightIndex,
		FightObservedStartT:               ctx.FightObservedStartT,
		FightObservedEndT:                 ctx.FightObservedEndT,
		WindowStartT:                      ctx.WindowStartT,
		WindowEndT:                        ctx.WindowEndT,
		WindowEndReason:                   ctx.WindowEndReason,
		WindowDurationSeconds:             ctx.WindowDurationSeconds,
		TargetInvolved:                    ctx.TargetInvolved,
		TargetEndSampleAvailable:          ctx.TargetEndSampleAvailable,
		TargetAliveAtEnd:                  ctx.TargetAliveAtEnd,
		AlliedEndSamplesAvailable:         ctx.AlliedEndSamplesAvailable,
		AlliedHeroesAliveAtEnd:            ctx.AlliedHeroesAliveAtEnd,
		AlliedDeaths:                      ctx.AlliedDeaths,
		EnemyDeaths:                       ctx.EnemyDeaths,
		EnemyDeathAdvantage:               ctx.EnemyDeathAdvantage,
		EnemyDeathWindowEndStateAvailable: deathStateAvailable,
		EnemyDeathVictimSlots:             deathSlots,
		EnemyDeathsStillDeadAtWindowEnd:   stillDead,
		EnemyTier1sDestroyedAtEnd:         destroyedT1s,
		EnemyMapOpened:                    len(destroyedT1s) > 0,
		EnemyFrontTowerOptions:            frontTowers,
		EnemyPushableTowerOptions:         pushableTowers,
		RoshanKnowledgeState:              ctx.RoshanAtEnd.KnowledgeState,
		RoshanKnownAliveForDecision:       ctx.RoshanAtEnd.KnownAliveForDecision,
		KnownObjectiveOptionCount:         knownObjectiveOptionCount,
		KnownObjectiveOptions:             knownObjectiveOptionCount > 0,
		TargetTeamConversionCount:         len(ctx.TargetTeamConversions),
		NoTargetTeamConversion:            len(ctx.TargetTeamConversions) == 0,
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

func sortObjectiveTowerOptions(options []ObjectiveTowerOption) {
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Lane == options[j].Lane {
			return options[i].Tier < options[j].Tier
		}
		return options[i].Lane < options[j].Lane
	})
}

// objectivePushableTowerOptions applies exact Dota backdoor mechanics as a
// sufficient tower-availability filter. T1 is always unprotected. T2 requires
// conservative friendly lane-creep support and an allied hero inside its
// independent 900-unit detection region during the clean post-fight interval.
// T3 uses the shared base state centered on the enemy Fort/Ancient and requires
// both creep support and allied hero presence inside that 4000-unit region.
// Creep evidence in the preceding 15 seconds is accepted because the game keeps
// protection disabled for that duration after qualifying creeps leave/die.
func objectivePushableTowerOptions(tl *timeline.MatchTimeline, ctx timeline.PostFightObjectiveContext, front []ObjectiveTowerOption) []ObjectiveTowerOption {
	out := make([]ObjectiveTowerOption, 0, len(front))
	if tl == nil {
		return out
	}
	enemyTeam := objectiveOpposingTeam(ctx.TargetTeam)
	if enemyTeam == 0 {
		return out
	}

	creepWindowStart := ctx.FightObservedEndT - objectiveBackdoorDisableSeconds
	if creepWindowStart < 0 {
		creepWindowStart = 0
	}
	heroWindowStart := ctx.WindowStartT
	if heroWindowStart < ctx.FightObservedEndT {
		heroWindowStart = ctx.FightObservedEndT
	}
	for _, option := range front {
		switch option.Tier {
		case 1:
			out = append(out, option)
		case 2:
			x, y, ok := objectiveInitialTowerPosition(tl.LaneStructures, enemyTeam, option.Lane, 2)
			if ok &&
				objectiveCreepBackdoorSupport(tl.CreepClusters, ctx.TargetTeam, x, y, objectiveT2BackdoorRadiusWorld, creepWindowStart, ctx.WindowEndT) &&
				objectiveAlliedHeroPresence(tl, ctx.TargetTeam, x, y, objectiveT2BackdoorRadiusWorld, heroWindowStart, ctx.WindowEndT) {
				out = append(out, option)
			}
		case 3:
			x, y, ok := objectiveInitialFortPosition(tl.LaneStructures, enemyTeam)
			if ok &&
				objectiveCreepBackdoorSupport(tl.CreepClusters, ctx.TargetTeam, x, y, objectiveBaseBackdoorRadiusWorld, creepWindowStart, ctx.WindowEndT) &&
				objectiveAlliedHeroPresence(tl, ctx.TargetTeam, x, y, objectiveBaseBackdoorRadiusWorld, heroWindowStart, ctx.WindowEndT) {
				out = append(out, option)
			}
		}
	}
	sortObjectiveTowerOptions(out)
	return out
}

func objectiveInitialTowerPosition(structures timeline.LaneStructureTimeline, team int, lane string, tier int) (float64, float64, bool) {
	for _, tower := range structures.InitialTowers {
		if tower.Team == team && tower.Lane == lane && tower.Tier == tier {
			return tower.X, tower.Y, true
		}
	}
	return 0, 0, false
}

func objectiveInitialFortPosition(structures timeline.LaneStructureTimeline, team int) (float64, float64, bool) {
	for _, fort := range structures.InitialForts {
		if fort.Team == team {
			return fort.X, fort.Y, true
		}
	}
	return 0, 0, false
}

// objectiveCreepBackdoorSupport intentionally uses a sufficient geometric
// condition rather than estimating individual creep positions from a cluster
// center. The whole observed same-team lane/siege cluster must fit inside the
// mechanic radius (center distance + max member spread <= radius). This can
// miss real support but cannot create support merely because an averaged center
// happens to fall inside the radius.
func objectiveCreepBackdoorSupport(clusters timeline.CreepClusterTimeline, team int, x, y, radiusWorld, startT, endT float64) bool {
	if !clusters.Available || (team != 2 && team != 3) || endT < startT {
		return false
	}
	for _, frame := range clusters.Frames {
		if frame.T < startT {
			continue
		}
		if frame.T > endT {
			break
		}
		for _, cluster := range frame.Clusters {
			if cluster.Team != team || cluster.CreepCount <= 0 || cluster.LaneCreepCount+cluster.SiegeCreepCount <= 0 {
				continue
			}
			centerDistanceWorld := math.Hypot(cluster.CenterX-x, cluster.CenterY-y) * objectiveWorldScale
			if centerDistanceWorld+cluster.MaxMemberDistanceWorld <= radiusWorld {
				return true
			}
		}
	}
	return false
}

// objectiveAlliedHeroPresence uses only allied hero positions, which are
// player-known. The radius is the same mechanic-defined backdoor region used by
// the protected objective; no separate coaching-distance threshold is added.
func objectiveAlliedHeroPresence(tl *timeline.MatchTimeline, team int, x, y, radiusWorld, startT, endT float64) bool {
	if tl == nil || (team != 2 && team != 3) || endT < startT {
		return false
	}
	for _, player := range tl.Players {
		if player == nil || player.Team != team {
			continue
		}
		for _, sample := range player.Samples {
			if sample.T < startT {
				continue
			}
			if sample.T > endT {
				break
			}
			if !sample.Alive {
				continue
			}
			if math.Hypot(sample.X-x, sample.Y-y)*objectiveWorldScale <= radiusWorld {
				return true
			}
		}
	}
	return false
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
