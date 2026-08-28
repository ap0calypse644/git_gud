package timeline

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
