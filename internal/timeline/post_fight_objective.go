package timeline

import "sort"

const postFightObjectiveMethod = "observed_fight_deaths_objective_events_and_causal_roshan_knowledge_v1"

// PostFightObjectiveTimeline is evidence-only context for what happened after
// each consolidated fight. It deliberately does not claim that an objective
// should have been taken or that failing to take one was a mistake.
type PostFightObjectiveTimeline struct {
	Available bool                        `json:"available"`
	Method    string                      `json:"method"`
	Contexts  []PostFightObjectiveContext `json:"contexts"`
}

// PostFightObjectiveContext starts at the final observed combat moment of one
// fight and ends at the next observed fight start (or match end). This avoids a
// hand-tuned number of seconds for an "objective window" while preserving the
// actual quiet period available for later calibration. If another distinct
// fight is already active at this fight's observed end, there is no clean
// post-fight interval and the window closes immediately.
type PostFightObjectiveContext struct {
	FightIndex              int     `json:"fight_index"`
	ObservedTimingAvailable bool    `json:"observed_timing_available"`
	FightObservedStartT     float64 `json:"fight_observed_start_t,omitempty"`
	FightObservedEndT       float64 `json:"fight_observed_end_t,omitempty"`
	WindowEndT              float64 `json:"window_end_t,omitempty"`
	WindowEndReason         string  `json:"window_end_reason,omitempty"` // next_fight_start | overlapping_fight_active | match_end
	WindowDurationSeconds   float64 `json:"window_duration_seconds,omitempty"`

	TargetTeam      int   `json:"target_team,omitempty"`
	TargetInvolved  bool  `json:"target_involved"`
	Participants    []int `json:"participants"`
	FightDeaths     int   `json:"fight_deaths"`
	FightHeroDamage int64 `json:"fight_hero_damage"`

	AlliedDeaths         int `json:"allied_deaths"`
	EnemyDeaths          int `json:"enemy_deaths"`
	EnemyDeathAdvantage  int `json:"enemy_death_advantage"`
	UnattributedDeaths   int `json:"unattributed_deaths"`

	TargetEndSampleAvailable  bool    `json:"target_end_sample_available"`
	TargetEndSampleT          float64 `json:"target_end_sample_t,omitempty"`
	TargetEndSampleAge        float64 `json:"target_end_sample_age_seconds,omitempty"`
	TargetAliveAtEnd          bool    `json:"target_alive_at_end,omitempty"`
	AlliedEndSamplesAvailable int     `json:"allied_end_samples_available"`
	AlliedHeroesAliveAtEnd    int     `json:"allied_heroes_alive_at_end"`

	EnemyLaneStructuresAtEnd []LaneStructureState `json:"enemy_lane_structures_at_end"`
	RoshanAtEnd              RoshanPostFightState  `json:"roshan_at_end"`

	TargetTeamConversions  []PostFightObjectiveEvent `json:"target_team_conversions"`
	EnemyTeamConversions   []PostFightObjectiveEvent `json:"enemy_team_conversions"`
	UnattributedConversions int                      `json:"unattributed_conversions"`
}

// RoshanPostFightState explicitly separates replay truth from information that
// may reasonably be treated as known. Exact random Roshan respawn is hidden
// game state: after a replay-observed respawn, WorldState may be "alive" while
// KnowledgeState remains "unknown_after_random_respawn".
type RoshanPostFightState struct {
	ReplayStateAvailable  bool     `json:"replay_state_available"`
	WorldState            string   `json:"world_state,omitempty"`     // alive | dead
	KnowledgeState        string   `json:"knowledge_state,omitempty"` // known_alive_from_game_start | known_dead_from_kill | unknown_after_random_respawn
	KnownAliveForDecision bool     `json:"known_alive_for_decision"`
	LastKillT             *float64 `json:"last_kill_t,omitempty"`
	LastSpawnT            *float64 `json:"last_spawn_t,omitempty"`
	LastKillerTeam        int      `json:"last_killer_team,omitempty"`
}

type PostFightObjectiveEvent struct {
	T      float64 `json:"t"`
	Type   string  `json:"type"`
	Team   int     `json:"team"`
	Actor  string  `json:"actor,omitempty"`
	Target string  `json:"target,omitempty"`
}

func DerivePostFightObjectiveTimeline(tl *MatchTimeline) PostFightObjectiveTimeline {
	out := PostFightObjectiveTimeline{
		Method:   postFightObjectiveMethod,
		Contexts: []PostFightObjectiveContext{},
	}
	if tl == nil {
		return out
	}
	target := tl.Players[slotKey(tl.TargetPlayerSlot)]
	if target == nil {
		return out
	}
	out.Available = true

	for fightIndex, fight := range tl.Fights {
		ctx := PostFightObjectiveContext{
			FightIndex:               fightIndex,
			ObservedTimingAvailable:  observedFightTimingAvailable(fight),
			TargetTeam:               target.Team,
			TargetInvolved:           fightContainsSlot(fight, tl.TargetPlayerSlot),
			Participants:             append([]int(nil), fight.Participants...),
			FightDeaths:              fight.Deaths,
			FightHeroDamage:          fight.HeroDamage,
			EnemyLaneStructuresAtEnd: []LaneStructureState{},
			TargetTeamConversions:    []PostFightObjectiveEvent{},
			EnemyTeamConversions:     []PostFightObjectiveEvent{},
		}
		if !ctx.ObservedTimingAvailable {
			out.Contexts = append(out.Contexts, ctx)
			continue
		}

		ctx.FightObservedStartT = fight.ObservedStartT
		ctx.FightObservedEndT = fight.ObservedEndT
		ctx.WindowEndT, ctx.WindowEndReason = nextPostFightBoundary(tl, fightIndex, fight.ObservedEndT)
		if ctx.WindowEndT < fight.ObservedEndT {
			ctx.WindowEndT = fight.ObservedEndT
		}
		ctx.WindowDurationSeconds = ctx.WindowEndT - fight.ObservedEndT

		populatePostFightDeaths(tl, fightIndex, target.Team, &ctx)
		populatePostFightAlliedState(tl, target.Team, tl.TargetPlayerSlot, fight.ObservedEndT, &ctx)
		ctx.EnemyLaneStructuresAtEnd = enemyLaneStructureStatesAt(tl, target.Team, fight.ObservedEndT)
		ctx.RoshanAtEnd = roshanPostFightStateAt(tl.Objectives, fight.ObservedEndT)
		populatePostFightConversions(tl.Objectives, target.Team, fight.ObservedEndT, ctx.WindowEndT, ctx.WindowEndReason, &ctx)
		out.Contexts = append(out.Contexts, ctx)
	}

	sort.SliceStable(out.Contexts, func(i, j int) bool {
		if out.Contexts[i].FightObservedEndT != out.Contexts[j].FightObservedEndT {
			return out.Contexts[i].FightObservedEndT < out.Contexts[j].FightObservedEndT
		}
		if out.Contexts[i].FightObservedStartT != out.Contexts[j].FightObservedStartT {
			return out.Contexts[i].FightObservedStartT < out.Contexts[j].FightObservedStartT
		}
		return out.Contexts[i].FightIndex < out.Contexts[j].FightIndex
	})
	return out
}

// nextPostFightBoundary is deliberately independent of tl.Fights slice order.
// Consolidated fights can overlap spatially and are not guaranteed to be
// ordered by observed timing. A clean post-fight objective interval therefore
// exists only if no other fight remains active at endT. Otherwise the window
// closes at endT. When the map is quiet at endT, choose the earliest observed
// start at or after endT across every other fight.
func nextPostFightBoundary(tl *MatchTimeline, fightIndex int, endT float64) (float64, string) {
	if tl == nil {
		return endT, "match_end"
	}

	boundary := tl.DurationSeconds
	reason := "match_end"
	for i, fight := range tl.Fights {
		if i == fightIndex || !observedFightTimingAvailable(fight) {
			continue
		}

		if fight.ObservedStartT < endT && fight.ObservedEndT > endT {
			return endT, "overlapping_fight_active"
		}
		if fight.ObservedStartT >= endT && fight.ObservedStartT < boundary {
			boundary = fight.ObservedStartT
			reason = "next_fight_start"
		}
	}
	return boundary, reason
}

func populatePostFightDeaths(tl *MatchTimeline, fightIndex, targetTeam int, ctx *PostFightObjectiveContext) {
	if ctx == nil {
		return
	}
	for _, event := range tl.Deaths {
		if event.T < ctx.FightObservedStartT || event.T > ctx.FightObservedEndT || event.VictimSlot == nil {
			continue
		}
		victim := tl.Players[slotKey(*event.VictimSlot)]
		if victim == nil {
			ctx.UnattributedDeaths++
			continue
		}
		if !eventBelongsToFight(tl, fightIndex, event.T, victim.PlayerSlot) {
			continue
		}
		switch victim.Team {
		case targetTeam:
			ctx.AlliedDeaths++
		case opposingTeam(targetTeam):
			ctx.EnemyDeaths++
		default:
			ctx.UnattributedDeaths++
		}
	}
	ctx.EnemyDeathAdvantage = ctx.EnemyDeaths - ctx.AlliedDeaths
}

func populatePostFightAlliedState(tl *MatchTimeline, targetTeam, targetSlot int, t float64, ctx *PostFightObjectiveContext) {
	if ctx == nil {
		return
	}
	for _, player := range tl.Players {
		if player == nil || player.Team != targetTeam {
			continue
		}
		sample, ok := heroSampleAtOrBefore(player, t)
		if !ok {
			continue
		}
		ctx.AlliedEndSamplesAvailable++
		if sample.Alive {
			ctx.AlliedHeroesAliveAtEnd++
		}
		if player.PlayerSlot == targetSlot {
			ctx.TargetEndSampleAvailable = true
			ctx.TargetEndSampleT = sample.T
			ctx.TargetEndSampleAge = t - sample.T
			ctx.TargetAliveAtEnd = sample.Alive
		}
	}
}

func enemyLaneStructureStatesAt(tl *MatchTimeline, targetTeam int, t float64) []LaneStructureState {
	enemyTeam := opposingTeam(targetTeam)
	if enemyTeam == 0 {
		return []LaneStructureState{}
	}
	out := make([]LaneStructureState, 0, 3)
	for _, lane := range []string{"top", "mid", "bottom"} {
		if state, ok := LaneStructureStateAt(tl.LaneStructures, enemyTeam, lane, t); ok {
			out = append(out, state)
		}
	}
	return out
}

func roshanPostFightStateAt(objectives []ObjectiveEvent, t float64) RoshanPostFightState {
	state := RoshanPostFightState{}
	for _, event := range objectives {
		if event.T > t {
			break
		}
		switch event.Type {
		case "roshan_alive_at_start":
			state.ReplayStateAvailable = true
			state.WorldState = "alive"
			state.KnowledgeState = "known_alive_from_game_start"
			state.KnownAliveForDecision = true
		case "roshan_kill":
			state.ReplayStateAvailable = true
			state.WorldState = "dead"
			state.KnowledgeState = "known_dead_from_kill"
			state.KnownAliveForDecision = false
			killT := event.T
			state.LastKillT = &killT
			state.LastKillerTeam = event.AttackerTeam
		case "roshan_spawned":
			state.ReplayStateAvailable = true
			state.WorldState = "alive"
			state.KnowledgeState = "unknown_after_random_respawn"
			state.KnownAliveForDecision = false
			spawnT := event.T
			state.LastSpawnT = &spawnT
		}
	}
	return state
}

func populatePostFightConversions(objectives []ObjectiveEvent, targetTeam int, startT, endT float64, endReason string, ctx *PostFightObjectiveContext) {
	if ctx == nil {
		return
	}
	for _, event := range objectives {
		if event.T <= startT {
			continue
		}
		if endReason == "next_fight_start" || endReason == "overlapping_fight_active" {
			if event.T >= endT {
				break
			}
		} else if event.T > endT {
			break
		}
		if event.Type != "building_kill" && event.Type != "roshan_kill" {
			continue
		}
		if event.AttackerTeam != 2 && event.AttackerTeam != 3 {
			ctx.UnattributedConversions++
			continue
		}
		converted := PostFightObjectiveEvent{
			T: event.T, Type: event.Type, Team: event.AttackerTeam, Actor: event.Actor, Target: event.Target,
		}
		if event.AttackerTeam == targetTeam {
			ctx.TargetTeamConversions = append(ctx.TargetTeamConversions, converted)
		} else if event.AttackerTeam == opposingTeam(targetTeam) {
			ctx.EnemyTeamConversions = append(ctx.EnemyTeamConversions, converted)
		}
	}
}

func opposingTeam(team int) int {
	switch team {
	case 2:
		return 3
	case 3:
		return 2
	default:
		return 0
	}
}
