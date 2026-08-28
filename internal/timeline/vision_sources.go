package timeline

import (
	"sort"

	"github.com/dotabuff/manta"
)

type wardEntityKey struct {
	index  int32
	serial int32
}

type rawWardInterval struct {
	key              wardEntityKey
	kind             string
	team             int
	ownerRawPlayerID *int
	x                float64
	y                float64
	dayVisionRange   float64
	nightVisionRange float64
	fowTeam          int
	startAbs         float64
	endAbs           float64
	endReason        string
	closed           bool
}

type wardCollector struct {
	active    map[wardEntityKey]int
	lastState map[wardEntityKey]int
	intervals []rawWardInterval
}

func newWardCollector() *wardCollector {
	return &wardCollector{
		active:    make(map[wardEntityKey]int),
		lastState: make(map[wardEntityKey]int),
	}
}

func wardKind(className string) (string, bool) {
	switch className {
	case "CDOTA_NPC_Observer_Ward", "DT_DOTA_NPC_Observer_Ward":
		return "observer", true
	case "CDOTA_NPC_Observer_Ward_TrueSight", "DT_DOTA_NPC_Observer_Ward_TrueSight":
		return "sentry", true
	default:
		return "", false
	}
}

// observe records ward lifecycle from replay entity state. It intentionally
// keeps raw replay facts only: a life-state transition tells us when a ward
// stopped being active, but not whether it was killed or naturally expired.
func (c *wardCollector) observe(e *manta.Entity, op manta.EntityOp, absoluteTime float64) {
	kind, ok := wardKind(e.GetClassName())
	if !ok {
		return
	}

	key := wardEntityKey{index: e.GetIndex(), serial: e.GetSerial()}

	if op.Flag(manta.EntityOpDeleted) || op.Flag(manta.EntityOpLeft) {
		c.close(key, absoluteTime, "entity_left")
		if op.Flag(manta.EntityOpDeleted) {
			delete(c.lastState, key)
		}
		return
	}

	lifeState, ok := numberInt(e.Get("m_lifeState"))
	if !ok {
		return
	}
	oldState, seen := c.lastState[key]
	c.lastState[key] = lifeState

	if lifeState == 0 {
		if _, active := c.active[key]; !active {
			c.open(e, key, kind, absoluteTime)
		}
		return
	}

	if seen && oldState == 0 {
		c.close(key, absoluteTime, "life_state_ended")
	}
}

func (c *wardCollector) open(e *manta.Entity, key wardEntityKey, kind string, absoluteTime float64) {
	team, _ := numberInt(e.Get("m_iTeamNum"))
	fowTeam, _ := numberInt(e.Get("m_nFoWTeam"))
	if team != dotaTeamRadiant && team != dotaTeamDire {
		if fowTeam == dotaTeamRadiant || fowTeam == dotaTeamDire {
			team = fowTeam
		}
	}

	x, _ := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
	y, _ := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
	dayVision, _ := numberFloat(e.Get("m_iDayTimeVisionRange"))
	nightVision, _ := numberFloat(e.Get("m_iNightTimeVisionRange"))

	var owner *int
	for _, field := range []string{"m_nPlayerOwnerID", "m_iPlayerOwnerID", "m_nControllingPlayerID"} {
		if v, ok := numberInt(e.Get(field)); ok && v >= 0 {
			vv := v
			owner = &vv
			break
		}
	}

	idx := len(c.intervals)
	c.intervals = append(c.intervals, rawWardInterval{
		key:              key,
		kind:             kind,
		team:             team,
		ownerRawPlayerID: owner,
		x:                x,
		y:                y,
		dayVisionRange:   dayVision,
		nightVisionRange: nightVision,
		fowTeam:          fowTeam,
		startAbs:         absoluteTime,
	})
	c.active[key] = idx
}

func (c *wardCollector) close(key wardEntityKey, absoluteTime float64, reason string) {
	idx, ok := c.active[key]
	if !ok {
		return
	}
	if idx >= 0 && idx < len(c.intervals) {
		c.intervals[idx].endAbs = absoluteTime
		c.intervals[idx].endReason = reason
		c.intervals[idx].closed = true
	}
	delete(c.active, key)
}

// apply normalizes absolute replay times onto match time and closes wards that
// survived until the end of the replay. Negative pre-horn placement times are
// clamped to zero so an already-active ward is represented at match start.
func (c *wardCollector) apply(out *MatchTimeline, gameStartAbs, duration float64) {
	out.VisionSources.Wards = make([]WardInterval, 0, len(c.intervals))
	matchEndAbs := gameStartAbs + duration

	for _, raw := range c.intervals {
		endAbs := raw.endAbs
		reason := raw.endReason
		if !raw.closed || endAbs <= 0 || endAbs > matchEndAbs {
			endAbs = matchEndAbs
			reason = "game_end"
		}

		start := raw.startAbs - gameStartAbs
		end := endAbs - gameStartAbs
		if end < 0 || start > duration {
			continue
		}
		if start < 0 {
			start = 0
		}
		if end > duration {
			end = duration
		}
		if end < start {
			continue
		}

		out.VisionSources.Wards = append(out.VisionSources.Wards, WardInterval{
			EntityIndex:      raw.key.index,
			EntitySerial:     raw.key.serial,
			Kind:             raw.kind,
			Team:             raw.team,
			OwnerRawPlayerID: raw.ownerRawPlayerID,
			X:                raw.x,
			Y:                raw.y,
			StartT:           start,
			EndT:             end,
			EndReason:        reason,
			DayVisionRange:   raw.dayVisionRange,
			NightVisionRange: raw.nightVisionRange,
			FOWTeam:          raw.fowTeam,
		})
	}

	sort.Slice(out.VisionSources.Wards, func(i, j int) bool {
		if out.VisionSources.Wards[i].StartT == out.VisionSources.Wards[j].StartT {
			return out.VisionSources.Wards[i].EntityIndex < out.VisionSources.Wards[j].EntityIndex
		}
		return out.VisionSources.Wards[i].StartT < out.VisionSources.Wards[j].StartT
	})
}
