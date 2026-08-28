package timeline

import (
	"math"
	"sort"
	"strings"

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

type rawHeroVisionSample struct {
	absoluteT       float64
	playerSlot      int
	team            int
	x               float64
	y               float64
	alive           bool
	dayVisionRange  float64
	nightVisionRange float64
}

type wardCollector struct {
	active               map[wardEntityKey]int
	lastState            map[wardEntityKey]int
	intervals            []rawWardInterval
	heroSamples          []rawHeroVisionSample
	lastHeroSampleSecond map[int]int
}

func newWardCollector() *wardCollector {
	return &wardCollector{
		active:               make(map[wardEntityKey]int),
		lastState:            make(map[wardEntityKey]int),
		lastHeroSampleSecond: make(map[int]int),
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

// observe records replay-observed vision-source facts. Ward lifecycle is kept
// exact at entity-update granularity. Primary hero vision sources are sampled at
// roughly 1 Hz and intentionally exclude illusions/clones for this first model.
func (c *wardCollector) observe(e *manta.Entity, op manta.EntityOp, absoluteTime float64) {
	if strings.HasPrefix(e.GetClassName(), "CDOTA_Unit_Hero_") {
		c.observeHeroVision(e, op, absoluteTime)
	}

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

func (c *wardCollector) observeHeroVision(e *manta.Entity, op manta.EntityOp, absoluteTime float64) {
	if !op.Flag(manta.EntityOpUpdated) && !op.Flag(manta.EntityOpCreated) {
		return
	}
	if illusion, ok := e.GetBool("m_bIsIllusion"); ok && illusion {
		return
	}
	if clone, ok := e.GetBool("m_bIsClone"); ok && clone {
		return
	}

	rawPlayerID, ok := numberInt(e.Get("m_iPlayerID"))
	if !ok {
		return
	}
	team, ok := numberInt(e.Get("m_iTeamNum"))
	if !ok {
		return
	}
	slot, ok := heroPlayerSlot(rawPlayerID, team)
	if !ok {
		return
	}

	second := int(math.Floor(absoluteTime))
	if last, seen := c.lastHeroSampleSecond[slot]; seen && last == second {
		return
	}

	x, xOK := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
	y, yOK := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
	if !xOK || !yOK {
		return
	}
	dayVision, _ := numberFloat(e.Get("m_iDayTimeVisionRange"))
	nightVision, _ := numberFloat(e.Get("m_iNightTimeVisionRange"))
	if dayVision <= 0 && nightVision <= 0 {
		return
	}

	alive := true
	if lifeState, ok := numberInt(e.Get("m_lifeState")); ok {
		alive = lifeState == 0
	} else if hp, ok := numberInt(e.Get("m_iHealth")); ok {
		alive = hp > 0
	}

	c.heroSamples = append(c.heroSamples, rawHeroVisionSample{
		absoluteT:        absoluteTime,
		playerSlot:       slot,
		team:             team,
		x:                x,
		y:                y,
		alive:            alive,
		dayVisionRange:   dayVision,
		nightVisionRange: nightVision,
	})
	c.lastHeroSampleSecond[slot] = second
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

	out.VisionSources.Heroes = make([]HeroVisionSample, 0, len(c.heroSamples))
	for _, raw := range c.heroSamples {
		t := raw.absoluteT - gameStartAbs
		if t < 0 || t > duration {
			continue
		}
		out.VisionSources.Heroes = append(out.VisionSources.Heroes, HeroVisionSample{
			T:                t,
			PlayerSlot:       raw.playerSlot,
			Team:             raw.team,
			X:                raw.x,
			Y:                raw.y,
			Alive:            raw.alive,
			DayVisionRange:   raw.dayVisionRange,
			NightVisionRange: raw.nightVisionRange,
		})
	}
	sort.Slice(out.VisionSources.Heroes, func(i, j int) bool {
		if out.VisionSources.Heroes[i].T == out.VisionSources.Heroes[j].T {
			return out.VisionSources.Heroes[i].PlayerSlot < out.VisionSources.Heroes[j].PlayerSlot
		}
		return out.VisionSources.Heroes[i].T < out.VisionSources.Heroes[j].T
	})
}
