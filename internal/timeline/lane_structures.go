package timeline

import (
	"sort"
	"strings"
)

const laneStructureMethod = "combat_log_lane_tower_destruction_v1"

// LaneStructureTimeline contains only replay-observed destruction facts for
// lane towers. Absence of a destruction event is intentionally not promoted to
// a claim that a tower is alive.
type LaneStructureTimeline struct {
	Available                bool                     `json:"available"`
	Method                   string                   `json:"method"`
	BuildingKillsObserved    int                      `json:"building_kills_observed"`
	LaneTowerKillsAccepted   int                      `json:"lane_tower_kills_accepted"`
	IgnoredNonLaneStructures int                      `json:"ignored_non_lane_structures"`
	RejectedMalformed        int                      `json:"rejected_malformed"`
	Events                   []LaneStructureEvent     `json:"events"`
}

// LaneStructureEvent is one observed lane tower destruction. Team is the team
// that owned the destroyed tower. Lane is top|mid|bottom and Tier is 1..3.
type LaneStructureEvent struct {
	T         float64 `json:"t"`
	Team      int     `json:"team"`
	Lane      string  `json:"lane"`
	Tier      int     `json:"tier"`
	RawTarget string  `json:"raw_target"`
}

// LaneStructureState is a causal point-in-time summary. The fields say only
// whether destruction has been observed by T; false means "not observed
// destroyed by T", not a stronger proof that the tower is alive.
type LaneStructureState struct {
	Team               int      `json:"team"`
	Lane               string   `json:"lane"`
	T                  float64  `json:"t"`
	Tier1Destroyed     bool     `json:"tier1_destroyed"`
	Tier2Destroyed     bool     `json:"tier2_destroyed"`
	Tier3Destroyed     bool     `json:"tier3_destroyed"`
	Tier1DestroyedAt   *float64 `json:"tier1_destroyed_at,omitempty"`
	Tier2DestroyedAt   *float64 `json:"tier2_destroyed_at,omitempty"`
	Tier3DestroyedAt   *float64 `json:"tier3_destroyed_at,omitempty"`
}

// DeriveLaneStructures normalizes combat-log building_kill targets into lane
// tower destruction facts. T4, barracks, fort and filler structures are
// deliberately excluded because they do not describe lane-front tower state.
func DeriveLaneStructures(tl *MatchTimeline) LaneStructureTimeline {
	out := LaneStructureTimeline{
		Method: laneStructureMethod,
		Events: []LaneStructureEvent{},
	}
	if tl == nil {
		return out
	}
	out.Available = true

	seen := make(map[[3]int]bool)
	for _, objective := range tl.Objectives {
		if objective.Type != "building_kill" {
			continue
		}
		out.BuildingKillsObserved++

		team, lane, tier, isLaneTower, malformed := parseLaneTowerTarget(objective.Target)
		if malformed {
			out.RejectedMalformed++
			continue
		}
		if !isLaneTower {
			out.IgnoredNonLaneStructures++
			continue
		}
		if objective.TargetTeam != 0 && objective.TargetTeam != team {
			out.RejectedMalformed++
			continue
		}

		key := [3]int{team, laneOrder(lane), tier}
		if seen[key] {
			continue
		}
		seen[key] = true
		out.Events = append(out.Events, LaneStructureEvent{
			T: objective.T, Team: team, Lane: lane, Tier: tier, RawTarget: objective.Target,
		})
	}

	sort.SliceStable(out.Events, func(i, j int) bool {
		if out.Events[i].T != out.Events[j].T {
			return out.Events[i].T < out.Events[j].T
		}
		if out.Events[i].Team != out.Events[j].Team {
			return out.Events[i].Team < out.Events[j].Team
		}
		if out.Events[i].Lane != out.Events[j].Lane {
			return laneOrder(out.Events[i].Lane) < laneOrder(out.Events[j].Lane)
		}
		return out.Events[i].Tier < out.Events[j].Tier
	})
	out.LaneTowerKillsAccepted = len(out.Events)
	return out
}

// LaneStructureStateAt returns the lane tower destructions observed at or
// before t. It never consumes a future destruction event.
func LaneStructureStateAt(timeline LaneStructureTimeline, team int, lane string, t float64) (LaneStructureState, bool) {
	lane = normalizeLaneName(lane)
	if !timeline.Available || (team != 2 && team != 3) || lane == "" {
		return LaneStructureState{}, false
	}
	state := LaneStructureState{Team: team, Lane: lane, T: t}
	for _, event := range timeline.Events {
		if event.T > t {
			break
		}
		if event.Team != team || event.Lane != lane {
			continue
		}
		t := event.T
		switch event.Tier {
		case 1:
			state.Tier1Destroyed = true
			state.Tier1DestroyedAt = &t
		case 2:
			state.Tier2Destroyed = true
			state.Tier2DestroyedAt = &t
		case 3:
			state.Tier3Destroyed = true
			state.Tier3DestroyedAt = &t
		}
	}
	return state, true
}

func parseLaneTowerTarget(target string) (team int, lane string, tier int, isLaneTower bool, malformed bool) {
	const goodPrefix = "npc_dota_goodguys_tower"
	const badPrefix = "npc_dota_badguys_tower"

	var rest string
	switch {
	case strings.HasPrefix(target, goodPrefix):
		team = 2
		rest = strings.TrimPrefix(target, goodPrefix)
	case strings.HasPrefix(target, badPrefix):
		team = 3
		rest = strings.TrimPrefix(target, badPrefix)
	default:
		return 0, "", 0, false, false
	}

	// tower4 has no lane suffix and is intentionally outside this model.
	if rest == "4" {
		return team, "", 0, false, false
	}
	if len(rest) < 3 || rest[1] != '_' {
		return 0, "", 0, false, true
	}
	switch rest[0] {
	case '1':
		tier = 1
	case '2':
		tier = 2
	case '3':
		tier = 3
	default:
		return 0, "", 0, false, true
	}
	lane = normalizeLaneName(rest[2:])
	if lane == "" {
		return 0, "", 0, false, true
	}
	return team, lane, tier, true, false
}

func normalizeLaneName(lane string) string {
	switch strings.ToLower(strings.TrimSpace(lane)) {
	case "top":
		return "top"
	case "mid", "middle":
		return "mid"
	case "bot", "bottom":
		return "bottom"
	default:
		return ""
	}
}
