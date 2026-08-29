package timeline

import "math"

const (
	laneProgressMethod    = "replay_lane_tower_polyline_v1"
	laneProgressWorldScale = 128.0
)

// LaneProgressGeometry is an auditable, replay-derived lane centerline proxy.
// Progress runs from the configured player's side toward the enemy side. Top
// and bottom add a bend inferred from the outer T2->T1 tower rays; mid uses the
// direct tower chain. No hard-coded map coordinates are used.
type LaneProgressGeometry struct {
	Lane         string              `json:"lane"`
	FriendlyTeam int                 `json:"friendly_team"`
	EnemyTeam    int                 `json:"enemy_team"`
	Method       string              `json:"method"`
	TotalWorld   float64             `json:"total_world"`
	Points       []LaneProgressPoint `json:"points"`
}

// LaneProgressPoint is one polyline landmark. ProgressWorld is cumulative from
// the friendly T3 along the geometry toward the enemy T3.
type LaneProgressPoint struct {
	Kind          string  `json:"kind"`
	Team          int     `json:"team,omitempty"`
	Tier          int     `json:"tier,omitempty"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
	ProgressWorld float64 `json:"progress_world"`
}

type laneProjection struct {
	ProgressWorld float64
	OffsetWorld   float64
}

func buildLaneProgressGeometry(positions []LaneTowerPosition, lane string, friendlyTeam int) (LaneProgressGeometry, bool) {
	lane = normalizeLaneName(lane)
	if lane == "" || (friendlyTeam != 2 && friendlyTeam != 3) {
		return LaneProgressGeometry{}, false
	}
	enemyTeam := 5 - friendlyTeam

	lookup := make(map[[2]int]LaneTowerPosition, 6)
	for _, position := range positions {
		if position.Lane != lane || (position.Team != friendlyTeam && position.Team != enemyTeam) || position.Tier < 1 || position.Tier > 3 {
			continue
		}
		lookup[[2]int{position.Team, position.Tier}] = position
	}
	for _, team := range []int{friendlyTeam, enemyTeam} {
		for tier := 1; tier <= 3; tier++ {
			if _, ok := lookup[[2]int{team, tier}]; !ok {
				return LaneProgressGeometry{}, false
			}
		}
	}

	friendlyT3 := lookup[[2]int{friendlyTeam, 3}]
	friendlyT2 := lookup[[2]int{friendlyTeam, 2}]
	friendlyT1 := lookup[[2]int{friendlyTeam, 1}]
	enemyT1 := lookup[[2]int{enemyTeam, 1}]
	enemyT2 := lookup[[2]int{enemyTeam, 2}]
	enemyT3 := lookup[[2]int{enemyTeam, 3}]

	points := []LaneProgressPoint{
		laneProgressTowerPoint("friendly_t3", friendlyT3),
		laneProgressTowerPoint("friendly_t2", friendlyT2),
		laneProgressTowerPoint("friendly_t1", friendlyT1),
	}
	if lane == "top" || lane == "bottom" {
		if x, y, ok := inferOuterLaneBend(friendlyT2, friendlyT1, enemyT1, enemyT2); ok {
			points = append(points, LaneProgressPoint{Kind: "lane_bend", X: x, Y: y})
		}
	}
	points = append(points,
		laneProgressTowerPoint("enemy_t1", enemyT1),
		laneProgressTowerPoint("enemy_t2", enemyT2),
		laneProgressTowerPoint("enemy_t3", enemyT3),
	)

	var cumulative float64
	for i := range points {
		if i > 0 {
			cumulative += math.Hypot(points[i].X-points[i-1].X, points[i].Y-points[i-1].Y) * laneProgressWorldScale
		}
		points[i].ProgressWorld = cumulative
	}
	if cumulative <= 0 {
		return LaneProgressGeometry{}, false
	}
	return LaneProgressGeometry{
		Lane: lane, FriendlyTeam: friendlyTeam, EnemyTeam: enemyTeam,
		Method: laneProgressMethod, TotalWorld: cumulative, Points: points,
	}, true
}

func laneProgressTowerPoint(kind string, position LaneTowerPosition) LaneProgressPoint {
	return LaneProgressPoint{
		Kind: kind, Team: position.Team, Tier: position.Tier,
		X: position.X, Y: position.Y,
	}
}

// inferOuterLaneBend intersects two outward tower rays: friendly T2->T1 and
// enemy T2->T1. On top/bottom this reconstructs the lane corner from replay
// geometry itself. If the rays do not meet forward of both T1s, callers fall
// back to a direct T1-to-T1 segment.
func inferOuterLaneBend(friendlyT2, friendlyT1, enemyT1, enemyT2 LaneTowerPosition) (float64, float64, bool) {
	px, py := friendlyT1.X, friendlyT1.Y
	rx, ry := friendlyT1.X-friendlyT2.X, friendlyT1.Y-friendlyT2.Y
	qx, qy := enemyT1.X, enemyT1.Y
	sx, sy := enemyT1.X-enemyT2.X, enemyT1.Y-enemyT2.Y

	denom := cross2(rx, ry, sx, sy)
	if math.Abs(denom) < 1e-6 {
		return 0, 0, false
	}
	qpx, qpy := qx-px, qy-py
	t := cross2(qpx, qpy, sx, sy) / denom
	u := cross2(qpx, qpy, rx, ry) / denom
	if t < -0.05 || u < -0.05 {
		return 0, 0, false
	}
	x, y := px+t*rx, py+t*ry
	if math.Hypot(x-friendlyT1.X, y-friendlyT1.Y) < 1e-6 || math.Hypot(x-enemyT1.X, y-enemyT1.Y) < 1e-6 {
		return 0, 0, false
	}
	return x, y, true
}

func cross2(ax, ay, bx, by float64) float64 { return ax*by - ay*bx }

func projectLaneProgress(geometry LaneProgressGeometry, x, y float64) (laneProjection, bool) {
	if len(geometry.Points) < 2 {
		return laneProjection{}, false
	}

	bestDistance2 := math.MaxFloat64
	bestProgress := 0.0
	for i := 0; i < len(geometry.Points)-1; i++ {
		a := geometry.Points[i]
		b := geometry.Points[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		length2 := dx*dx + dy*dy
		if length2 <= 0 {
			continue
		}
		u := ((x-a.X)*dx + (y-a.Y)*dy) / length2
		if u < 0 {
			u = 0
		} else if u > 1 {
			u = 1
		}
		projX, projY := a.X+u*dx, a.Y+u*dy
		distance2 := (x-projX)*(x-projX) + (y-projY)*(y-projY)
		if distance2 >= bestDistance2 {
			continue
		}
		bestDistance2 = distance2
		segmentWorld := math.Sqrt(length2) * laneProgressWorldScale
		bestProgress = a.ProgressWorld + u*segmentWorld
	}
	if bestDistance2 == math.MaxFloat64 {
		return laneProjection{}, false
	}
	return laneProjection{
		ProgressWorld: bestProgress,
		OffsetWorld:   math.Sqrt(bestDistance2) * laneProgressWorldScale,
	}, true
}

func laneProgressForTower(geometry LaneProgressGeometry, friendly bool, tier int) (float64, bool) {
	prefix := "enemy_t"
	if friendly {
		prefix = "friendly_t"
	}
	kind := prefix + string(rune('0'+tier))
	for _, point := range geometry.Points {
		if point.Kind == kind {
			return point.ProgressWorld, true
		}
	}
	return 0, false
}

// outermostNotObservedDestroyedTier returns the lane-front reference implied by
// M15 destruction evidence. It does not assert that the returned tower is
// alive; it only says its destruction has not yet been observed.
func outermostNotObservedDestroyedTier(state LaneStructureState) (int, bool) {
	if !state.Tier1Destroyed {
		return 1, true
	}
	if !state.Tier2Destroyed {
		return 2, true
	}
	if !state.Tier3Destroyed {
		return 3, true
	}
	return 0, false
}
