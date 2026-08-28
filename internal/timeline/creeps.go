package timeline

import (
	"math"
	"sort"

	"github.com/dotabuff/manta"
)

const (
	creepClusterMethod          = "same_team_lane_and_siege_creep_connected_components_1200_world_1hz"
	creepClusterRadiusWorld    = 1200.0
	creepClusterRadiusTimeline = creepClusterRadiusWorld / 128.0
)

type creepEntityKey struct {
	index  int32
	serial int32
}

type creepState struct {
	key          creepEntityKey
	team         int
	kind         string // lane | siege
	x            float64
	y            float64
	alive        bool
	waiting      bool
	cohortSecond int
	cohortKnown  bool
}

type creepCollector struct {
	states             map[creepEntityKey]creepState
	frames             []CreepClusterFrame
	waveFrames         []rawWaveFrame
	started            bool
	lastSnapshotSecond int
	validStateObserved bool
	activationEvidence LaneWaveActivationEvidence
}

func newCreepCollector() *creepCollector {
	return &creepCollector{states: make(map[creepEntityKey]creepState)}
}

func laneCreepKind(className string) (string, bool) {
	switch className {
	case "CDOTA_BaseNPC_Creep_Lane":
		return "lane", true
	case "CDOTA_BaseNPC_Creep_Siege":
		return "siege", true
	default:
		return "", false
	}
}

// observe consumes only the two real lane-wave entity classes validated in
// M11. Sampling is boundary-based: when the replay first advances into a new
// integer second, the collector snapshots the last creep state observed at or
// before that boundary, then applies the current update.
func (c *creepCollector) observe(e *manta.Entity, op manta.EntityOp, matchTime float64) {
	kind, ok := laneCreepKind(e.GetClassName())
	if !ok || matchTime < 0 {
		return
	}

	second := int(math.Floor(matchTime))
	c.advance(second)

	key := creepEntityKey{index: e.GetIndex(), serial: e.GetSerial()}
	if op.Flag(manta.EntityOpDeleted) || op.Flag(manta.EntityOpLeft) {
		delete(c.states, key)
		return
	}
	if !op.Flag(manta.EntityOpUpdated) && !op.Flag(manta.EntityOpCreated) {
		return
	}

	state, ok := readCreepState(e, key, kind)
	if !ok {
		// A patch may stop transmitting one of the fields validated by M11.
		// Preserve the last valid state rather than fabricating a replacement.
		return
	}

	previous, seen := c.states[key]
	var source string
	state, source = assignCreepActivation(previous, seen, state, op.Flag(manta.EntityOpCreated), matchTime)
	switch source {
	case "waiting_transition":
		c.activationEvidence.WaitingTransitions++
	case "created_active":
		c.activationEvidence.CreatedActive++
	case "first_observed_active":
		c.activationEvidence.FirstObservedActive++
	}

	c.validStateObserved = true
	c.states[key] = state
}

func assignCreepActivation(previous creepState, seen bool, state creepState, created bool, matchTime float64) (creepState, string) {
	if state.waiting {
		return state, ""
	}
	if seen && previous.cohortKnown {
		state.cohortSecond = previous.cohortSecond
		state.cohortKnown = true
		return state, ""
	}

	// Cohorts are an observed replay reconstruction, not a hard-coded Dota
	// spawn schedule. Rounding to the nearest second absorbs callback ordering
	// within one spawn without assuming a 30-second cadence.
	state.cohortSecond = int(math.Floor(matchTime + 0.5))
	state.cohortKnown = true

	if seen && previous.waiting {
		return state, "waiting_transition"
	}
	if created {
		return state, "created_active"
	}
	return state, "first_observed_active"
}

func readCreepState(e *manta.Entity, key creepEntityKey, kind string) (creepState, bool) {
	team, ok := numberInt(e.Get("m_iTeamNum"))
	if !ok || (team != 2 && team != 3) {
		return creepState{}, false
	}
	lifeState, ok := numberInt(e.Get("m_lifeState"))
	if !ok {
		return creepState{}, false
	}
	hp, ok := numberInt(e.Get("m_iHealth"))
	if !ok {
		return creepState{}, false
	}
	if _, ok := numberInt(e.Get("m_iMaxHealth")); !ok {
		return creepState{}, false
	}
	waiting, ok := e.GetBool("m_bIsWaitingToSpawn")
	if !ok {
		return creepState{}, false
	}
	x, xOK := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
	y, yOK := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
	if !xOK || !yOK {
		return creepState{}, false
	}

	return creepState{
		key:     key,
		team:    team,
		kind:    kind,
		x:       x,
		y:       y,
		alive:   lifeState == 0 && hp > 0,
		waiting: waiting,
	}, true
}

func (c *creepCollector) advance(second int) {
	if !c.started {
		c.started = true
		c.lastSnapshotSecond = second
		return
	}
	if second <= c.lastSnapshotSecond {
		return
	}
	for snapshotSecond := c.lastSnapshotSecond + 1; snapshotSecond <= second; snapshotSecond++ {
		c.frames = append(c.frames, CreepClusterFrame{
			T:        float64(snapshotSecond),
			Clusters: clusterCreepStates(c.states, creepClusterRadiusTimeline),
		})
		c.waveFrames = append(c.waveFrames, rawWaveFrame{
			T:          float64(snapshotSecond),
			Components: cohortCreepComponents(c.states, creepClusterRadiusTimeline),
		})
	}
	c.lastSnapshotSecond = second
}

func (c *creepCollector) finalize(duration float64) CreepClusterTimeline {
	if duration >= 0 {
		c.advance(int(math.Floor(duration)))
	}
	frames := c.frames
	if frames == nil {
		frames = []CreepClusterFrame{}
	}
	return CreepClusterTimeline{
		Available:             c.validStateObserved,
		Method:                creepClusterMethod,
		SampleIntervalSeconds: 1,
		ClusterRadiusWorld:    creepClusterRadiusWorld,
		ClusterRadiusTimeline: creepClusterRadiusTimeline,
		Frames:                frames,
	}
}

func (c *creepCollector) finalizeLaneWaves(duration float64) LaneWaveTimeline {
	if duration >= 0 {
		c.advance(int(math.Floor(duration)))
	}
	return deriveLaneWaveTimeline(c.waveFrames, c.activationEvidence, c.validStateObserved)
}

func clusterCreepStates(states map[creepEntityKey]creepState, radius float64) []CreepCluster {
	points := make([]creepState, 0, len(states))
	for _, state := range states {
		if !state.alive || state.waiting {
			continue
		}
		points = append(points, state)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].team != points[j].team {
			return points[i].team < points[j].team
		}
		if points[i].key.index != points[j].key.index {
			return points[i].key.index < points[j].key.index
		}
		return points[i].key.serial < points[j].key.serial
	})
	if len(points) == 0 {
		return []CreepCluster{}
	}

	parent := make([]int, len(points))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	radius2 := radius * radius
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].team != points[j].team {
				continue
			}
			dx := points[i].x - points[j].x
			dy := points[i].y - points[j].y
			if dx*dx+dy*dy <= radius2 {
				union(i, j)
			}
		}
	}

	members := make(map[int][]int)
	for i := range points {
		root := find(i)
		members[root] = append(members[root], i)
	}

	clusters := make([]CreepCluster, 0, len(members))
	for _, indexes := range members {
		var sumX, sumY float64
		laneCount, siegeCount := 0, 0
		for _, idx := range indexes {
			point := points[idx]
			sumX += point.x
			sumY += point.y
			switch point.kind {
			case "lane":
				laneCount++
			case "siege":
				siegeCount++
			}
		}
		centerX := sumX / float64(len(indexes))
		centerY := sumY / float64(len(indexes))
		maxDistance := 0.0
		for _, idx := range indexes {
			dx := points[idx].x - centerX
			dy := points[idx].y - centerY
			d := math.Hypot(dx, dy)
			if d > maxDistance {
				maxDistance = d
			}
		}
		clusters = append(clusters, CreepCluster{
			Team:                      points[indexes[0]].team,
			CenterX:                   centerX,
			CenterY:                   centerY,
			CreepCount:                len(indexes),
			LaneCreepCount:            laneCount,
			SiegeCreepCount:           siegeCount,
			MaxMemberDistanceTimeline: maxDistance,
			MaxMemberDistanceWorld:    maxDistance * 128.0,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Team != clusters[j].Team {
			return clusters[i].Team < clusters[j].Team
		}
		if clusters[i].CenterX != clusters[j].CenterX {
			return clusters[i].CenterX < clusters[j].CenterX
		}
		if clusters[i].CenterY != clusters[j].CenterY {
			return clusters[i].CenterY < clusters[j].CenterY
		}
		return clusters[i].CreepCount < clusters[j].CreepCount
	})
	return clusters
}
