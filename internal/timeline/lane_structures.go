package timeline

import (
	"sort"
	"strings"
)

const laneStructureMethod = "replay_observed_initial_lane_towers_and_combat_log_destruction_v2"

// LaneStructureTimeline combines two causal replay facts for lane towers:
// named T1-T3 tower entities observed in the replay's initial map state, and
// combat-log building-kill events that mark their destruction. The initial
// tower set is serialized because detector CLIs operate on saved timelines and
// must not need the raw replay or hidden parser-only geometry.
type LaneStructureTimeline struct {
	Available                bool                       `json:"available"`
	Method                   string                     `json:"method"`
	InitialLaneTowersObserved int                        `json:"initial_lane_towers_observed"`
	InitialTowers            []LaneStructureInitialTower `json:"initial_towers"`
	BuildingKillsObserved    int                        `json:"building_kills_observed"`
	LaneTowerKillsAccepted   int                        `json:"lane_tower_kills_accepted"`
	IgnoredNonLaneStructures int                        `json:"ignored_non_lane_structures"`
	RejectedMalformed        int                        `json:"rejected_malformed"`
	Events                   []LaneStructureEvent       `json:"events"`
}

// LaneStructureInitialTower is a public/static lane tower proven to exist by a
// named replay entity. X/Y are map geometry, not hidden enemy information.
type LaneStructureInitialTower struct {
	Team       int     `json:"team"`
	Lane       string  `json:"lane"`
	Tier       int     `json:"tier"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	EntityName string  `json:"entity_name"`
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

// LaneStructureState is a causal point-in-time summary. KnownAlive is stronger
// than the older "not observed destroyed" interpretation: it is true only when
// the replay established that named tower in the initial map state and no
// destruction event for it has occurred by T.
type LaneStructureState struct {
	Team               int      `json:"team"`
	Lane               string   `json:"lane"`
	T                  float64  `json:"t"`
	Tier1PresentAtStart bool     `json:"tier1_present_at_start"`
	Tier2PresentAtStart bool     `json:"tier2_present_at_start"`
	Tier3PresentAtStart bool     `json:"tier3_present_at_start"`
	Tier1KnownAlive    bool     `json:"tier1_known_alive"`
	Tier2KnownAlive    bool     `json:"tier2_known_alive"`
	Tier3KnownAlive    bool     `json:"tier3_known_alive"`
	Tier1Destroyed     bool     `json:"tier1_destroyed"`
	Tier2Destroyed     bool     `json:"tier2_destroyed"`
	Tier3Destroyed     bool     `json:"tier3_destroyed"`
	Tier1DestroyedAt   *float64 `json:"tier1_destroyed_at,omitempty"`
	Tier2DestroyedAt   *float64 `json:"tier2_destroyed_at,omitempty"`
	Tier3DestroyedAt   *float64 `json:"tier3_destroyed_at,omitempty"`
}

// DeriveLaneStructures normalizes replay-observed named T1-T3 entities and
// combat-log building_kill targets. T4, barracks, fort and filler structures
// are deliberately excluded because they do not describe lane-front state.
func DeriveLaneStructures(tl *MatchTimeline) LaneStructureTimeline {
	out := LaneStructureTimeline{
		Method:        laneStructureMethod,
		InitialTowers: []LaneStructureInitialTower{},
		Events:        []LaneStructureEvent{},
	}
	if tl == nil {
		return out
	}
	out.Available = true

	initialSeen := make(map[[3]int]bool)
	for _, tower := range tl.LaneTowerPositions {
		if tower.Team != 2 && tower.Team != 3 {
			continue
		}
		lane := normalizeLaneName(tower.Lane)
		if lane == "" || tower.Tier < 1 || tower.Tier > 3 {
			continue
		}
		key := [3]int{tower.Team, laneOrder(lane), tower.Tier}
		if initialSeen[key] {
			continue
		}
		initialSeen[key] = true
		out.InitialTowers = append(out.InitialTowers, LaneStructureInitialTower{
			Team: tower.Team, Lane: lane, Tier: tower.Tier, X: tower.X, Y: tower.Y, EntityName: tower.RawName,
		})
	}
	sort.SliceStable(out.InitialTowers, func(i, j int) bool {
		if out.InitialTowers[i].Team != out.InitialTowers[j].Team {
			return out.InitialTowers[i].Team < out.InitialTowers[j].Team
		}
		if out.InitialTowers[i].Lane != out.InitialTowers[j].Lane {
			return laneOrder(out.InitialTowers[i].Lane) < laneOrder(out.InitialTowers[j].Lane)
		}
		return out.InitialTowers[i].Tier < out.InitialTowers[j].Tier
	})
	out.InitialLaneTowersObserved = len(out.InitialTowers)

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

// LaneStructureStateAt returns causal tower state at or before t. A named
// initial tower is known alive until its combat-log destruction boundary; no
// future destruction event is consumed.
func LaneStructureStateAt(timeline LaneStructureTimeline, team int, lane string, t float64) (LaneStructureState, bool) {
	lane = normalizeLaneName(lane)
	if !timeline.Available || (team != 2 && team != 3) || lane == "" {
		return LaneStructureState{}, false
	}
	state := LaneStructureState{Team: team, Lane: lane, T: t}
	for _, tower := range timeline.InitialTowers {
		if tower.Team != team || tower.Lane != lane {
			continue
		}
		switch tower.Tier {
		case 1:
			state.Tier1PresentAtStart = true
			state.Tier1KnownAlive = true
		case 2:
			state.Tier2PresentAtStart = true
			state.Tier2KnownAlive = true
		case 3:
			state.Tier3PresentAtStart = true
			state.Tier3KnownAlive = true
		}
	}
	for _, event := range timeline.Events {
		if event.T > t {
			break
		}
		if event.Team != team || event.Lane != lane {
			continue
		}
		destroyedAt := event.T
		switch event.Tier {
		case 1:
			state.Tier1Destroyed = true
			state.Tier1KnownAlive = false
			state.Tier1DestroyedAt = &destroyedAt
		case 2:
			state.Tier2Destroyed = true
			state.Tier2KnownAlive = false
			state.Tier2DestroyedAt = &destroyedAt
		case 3:
			state.Tier3Destroyed = true
			state.Tier3KnownAlive = false
			state.Tier3DestroyedAt = &destroyedAt
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
