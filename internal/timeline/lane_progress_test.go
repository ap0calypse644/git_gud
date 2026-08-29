package timeline

import (
	"math"
	"testing"
)

func TestBuildLaneProgressGeometryBottomUsesTowerDerivedBend(t *testing.T) {
	positions := []LaneTowerPosition{
		{Team: 2, Lane: "bottom", Tier: 3, X: 0, Y: 0},
		{Team: 2, Lane: "bottom", Tier: 2, X: 10, Y: 0},
		{Team: 2, Lane: "bottom", Tier: 1, X: 20, Y: 0},
		{Team: 3, Lane: "bottom", Tier: 1, X: 30, Y: 10},
		{Team: 3, Lane: "bottom", Tier: 2, X: 30, Y: 20},
		{Team: 3, Lane: "bottom", Tier: 3, X: 30, Y: 30},
	}

	geometry, ok := buildLaneProgressGeometry(positions, "bottom", 2)
	if !ok {
		t.Fatal("expected bottom-lane geometry")
	}
	if len(geometry.Points) != 7 {
		t.Fatalf("points = %d, want 7: %#v", len(geometry.Points), geometry.Points)
	}
	bend := geometry.Points[3]
	if bend.Kind != "lane_bend" || math.Abs(bend.X-30) > 1e-9 || math.Abs(bend.Y) > 1e-9 {
		t.Fatalf("unexpected inferred bend: %#v", bend)
	}
	if math.Abs(geometry.TotalWorld-60*laneProgressWorldScale) > 1e-9 {
		t.Fatalf("total world = %f, want %f", geometry.TotalWorld, 60*laneProgressWorldScale)
	}

	projection, ok := projectLaneProgress(geometry, 30, 15)
	if !ok {
		t.Fatal("expected projection")
	}
	if math.Abs(projection.ProgressWorld-45*laneProgressWorldScale) > 1e-9 || math.Abs(projection.OffsetWorld) > 1e-9 {
		t.Fatalf("unexpected projection: %#v", projection)
	}
}

func TestBuildLaneProgressGeometryMidUsesDirectTowerChain(t *testing.T) {
	positions := []LaneTowerPosition{
		{Team: 2, Lane: "mid", Tier: 3, X: 0, Y: 0},
		{Team: 2, Lane: "mid", Tier: 2, X: 10, Y: 10},
		{Team: 2, Lane: "mid", Tier: 1, X: 20, Y: 20},
		{Team: 3, Lane: "mid", Tier: 1, X: 30, Y: 30},
		{Team: 3, Lane: "mid", Tier: 2, X: 40, Y: 40},
		{Team: 3, Lane: "mid", Tier: 3, X: 50, Y: 50},
	}
	geometry, ok := buildLaneProgressGeometry(positions, "mid", 2)
	if !ok {
		t.Fatal("expected mid-lane geometry")
	}
	if len(geometry.Points) != 6 {
		t.Fatalf("mid points = %d, want 6", len(geometry.Points))
	}
	for _, point := range geometry.Points {
		if point.Kind == "lane_bend" {
			t.Fatalf("mid lane must not invent a bend: %#v", geometry.Points)
		}
	}
}

func TestParseLaneTowerEntityNameAcceptsReplayEntityNames(t *testing.T) {
	team, lane, tier, isLaneTower, malformed := parseLaneTowerEntityName("dota_goodguys_tower1_bot")
	if malformed || !isLaneTower || team != 2 || lane != "bottom" || tier != 1 {
		t.Fatalf("unexpected parse: team=%d lane=%q tier=%d laneTower=%v malformed=%v", team, lane, tier, isLaneTower, malformed)
	}
}
