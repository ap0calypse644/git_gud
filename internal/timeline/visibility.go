package timeline

import "github.com/dotabuff/manta"

const (
	dotaTeamRadiant = 2
	dotaTeamDire    = 3
)

// visibleToTeam interprets m_iTaggedAsVisibleByTeam. Dota uses the team ID as
// the bit number: Radiant is bit 2 and Dire is bit 3.
func visibleToTeam(mask, team int) bool {
	if team < 0 || team >= 63 {
		return false
	}
	return mask&(1<<uint(team)) != 0
}

func makeVisibilityEvent(t float64, slot, team, mask int, x, y float64) VisibilityEvent {
	return VisibilityEvent{
		T:                 t,
		PlayerSlot:        slot,
		Team:              team,
		X:                 x,
		Y:                 y,
		VisibleByTeamMask: mask,
		VisibleToRadiant:  visibleToTeam(mask, dotaTeamRadiant),
		VisibleToDire:     visibleToTeam(mask, dotaTeamDire),
	}
}

type visibilityCollector struct {
	lastMask map[int]int
	seen     map[int]bool
}

func newVisibilityCollector() *visibilityCollector {
	return &visibilityCollector{
		lastMask: make(map[int]int),
		seen:     make(map[int]bool),
	}
}

// observe records a change only when the replay actually exposes
// m_iTaggedAsVisibleByTeam. It is called before the roughly-1Hz hero-snapshot
// throttle so a short visibility transition is not discarded merely because
// another hero update happened earlier in the same second.
func (c *visibilityCollector) observe(out *MatchTimeline, e *manta.Entity, slot, team int, t float64) {
	mask, ok := numberInt(e.Get("m_iTaggedAsVisibleByTeam"))
	if !ok {
		return
	}
	out.Visibility.DirectTeamMaskAvailable = true

	if c.seen[slot] && c.lastMask[slot] == mask {
		return
	}
	c.seen[slot] = true
	c.lastMask[slot] = mask

	x, _ := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
	y, _ := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
	out.Visibility.Events = append(out.Visibility.Events, makeVisibilityEvent(t, slot, team, mask, x, y))
}
