package timeline

import (
	"math"
	"sort"

	"github.com/dotabuff/manta"
)

const (
	// Source 2 uses the all-ones 24-bit entity handle as the invalid handle.
	// This exact value was also observed on the Roshan spawner between each
	// validated Roshan incarnation in replay 8960461734.
	invalidRoshanHandle       = uint64(0xFFFFFF)
	roshanEventSyncTolerance = 2.0 / tickRate
)

type rawRoshanSpawnerState struct {
	NetTick                 uint32
	AbsoluteT               float64
	Handle                  uint64
	KillCount               int
	LastKillerTeam          int
	LastKillerTeamAvailable bool
}

type roshanCollector struct {
	states []rawRoshanSpawnerState
	last   rawRoshanSpawnerState
	seen   bool
}

func newRoshanCollector() *roshanCollector {
	return &roshanCollector{}
}

func (c *roshanCollector) observe(e *manta.Entity, netTick uint32, absoluteTime float64) {
	if e.GetClassName() != "CDOTA_RoshanSpawner" {
		return
	}

	handle, handleOK := numberUint(e.Get("m_hRoshan"))
	killCount, killCountOK := numberInt(e.Get("m_iKillCount"))
	if !handleOK || !killCountOK {
		return
	}

	state := rawRoshanSpawnerState{
		NetTick:   netTick,
		AbsoluteT: absoluteTime,
		Handle:    handle,
		KillCount: killCount,
	}
	if team, ok := numberInt(e.Get("m_iLastKillerTeam")); ok {
		state.LastKillerTeam = team
		state.LastKillerTeamAvailable = true
	}

	if c.seen && sameRoshanSpawnerState(c.last, state) {
		return
	}
	c.states = append(c.states, state)
	c.last = state
	c.seen = true
}

func sameRoshanSpawnerState(a, b rawRoshanSpawnerState) bool {
	return a.Handle == b.Handle &&
		a.KillCount == b.KillCount &&
		a.LastKillerTeam == b.LastKillerTeam &&
		a.LastKillerTeamAvailable == b.LastKillerTeamAvailable
}

type roshanKillState struct {
	T          float64
	KillerTeam int
	TeamKnown  bool
}

func (c *roshanCollector) apply(out *MatchTimeline, gameStartTime float64) {
	if len(c.states) == 0 {
		return
	}

	states := append([]rawRoshanSpawnerState(nil), c.states...)
	sort.SliceStable(states, func(i, j int) bool { return states[i].NetTick < states[j].NetTick })

	var stateAtStart *rawRoshanSpawnerState
	for i := range states {
		t := states[i].AbsoluteT - gameStartTime
		if t > 0 {
			break
		}
		copy := states[i]
		stateAtStart = &copy
	}
	if stateAtStart != nil && stateAtStart.Handle != invalidRoshanHandle {
		out.Objectives = append(out.Objectives, ObjectiveEvent{
			T:      0,
			Type:   "roshan_alive_at_start",
			Target: "npc_dota_roshan",
		})
	}

	kills := make([]roshanKillState, 0)
	for i := 1; i < len(states); i++ {
		previous := states[i-1]
		current := states[i]
		t := current.AbsoluteT - gameStartTime
		if t < 0 {
			continue
		}

		if current.KillCount == previous.KillCount+1 {
			kills = append(kills, roshanKillState{
				T:          t,
				KillerTeam: current.LastKillerTeam,
				TeamKnown:  current.LastKillerTeamAvailable && (current.LastKillerTeam == 2 || current.LastKillerTeam == 3),
			})
		}

		if previous.Handle == invalidRoshanHandle && current.Handle != invalidRoshanHandle {
			out.Objectives = append(out.Objectives, ObjectiveEvent{
				T:      t,
				Type:   "roshan_spawned",
				Target: "npc_dota_roshan",
			})
		}
	}

	enrichRoshanKillTeams(out.Objectives, kills)
	sort.SliceStable(out.Objectives, func(i, j int) bool { return out.Objectives[i].T < out.Objectives[j].T })
}

func enrichRoshanKillTeams(objectives []ObjectiveEvent, kills []roshanKillState) {
	used := make([]bool, len(kills))
	for i := range objectives {
		if objectives[i].Type != "roshan_kill" || objectives[i].AttackerTeam != 0 {
			continue
		}

		best := -1
		bestDelta := math.Inf(1)
		for j := range kills {
			if used[j] || !kills[j].TeamKnown {
				continue
			}
			delta := math.Abs(objectives[i].T - kills[j].T)
			if delta <= roshanEventSyncTolerance && delta < bestDelta {
				best = j
				bestDelta = delta
			}
		}
		if best >= 0 {
			objectives[i].AttackerTeam = kills[best].KillerTeam
			used[best] = true
		}
	}
}
