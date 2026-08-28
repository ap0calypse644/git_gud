package timeline

import (
	"math"
	"testing"
)

func TestLaneCreepKind(t *testing.T) {
	tests := []struct {
		class string
		kind  string
		ok    bool
	}{
		{"CDOTA_BaseNPC_Creep_Lane", "lane", true},
		{"CDOTA_BaseNPC_Creep_Siege", "siege", true},
		{"CDOTA_BaseNPC_Creep_Neutral", "", false},
		{"CDOTA_Ability_Creep_Siege", "", false},
	}
	for _, tt := range tests {
		kind, ok := laneCreepKind(tt.class)
		if ok != tt.ok || kind != tt.kind {
			t.Fatalf("laneCreepKind(%q)=(%q,%v), want (%q,%v)", tt.class, kind, ok, tt.kind, tt.ok)
		}
	}
}

func TestClusterCreepStatesSeparatesTeamsAndDistantGroups(t *testing.T) {
	states := map[creepEntityKey]creepState{
		{index: 1, serial: 1}: {key: creepEntityKey{1, 1}, team: 2, kind: "lane", x: 10, y: 10, alive: true},
		{index: 2, serial: 1}: {key: creepEntityKey{2, 1}, team: 2, kind: "siege", x: 11, y: 10, alive: true},
		{index: 3, serial: 1}: {key: creepEntityKey{3, 1}, team: 2, kind: "lane", x: 40, y: 40, alive: true},
		{index: 4, serial: 1}: {key: creepEntityKey{4, 1}, team: 3, kind: "lane", x: 10.5, y: 10, alive: true},
	}

	clusters := clusterCreepStates(states, 3)
	if len(clusters) != 3 {
		t.Fatalf("cluster count=%d, want 3: %#v", len(clusters), clusters)
	}

	if clusters[0].Team != 2 || clusters[0].CreepCount != 2 || clusters[0].LaneCreepCount != 1 || clusters[0].SiegeCreepCount != 1 {
		t.Fatalf("first cluster=%+v, want radiant lane+siege pair", clusters[0])
	}
	if math.Abs(clusters[0].CenterX-10.5) > 1e-9 || math.Abs(clusters[0].CenterY-10) > 1e-9 {
		t.Fatalf("first center=(%f,%f), want (10.5,10)", clusters[0].CenterX, clusters[0].CenterY)
	}
	if clusters[1].Team != 2 || clusters[1].CreepCount != 1 {
		t.Fatalf("second cluster=%+v, want distant radiant singleton", clusters[1])
	}
	if clusters[2].Team != 3 || clusters[2].CreepCount != 1 {
		t.Fatalf("third cluster=%+v, want dire singleton", clusters[2])
	}
}

func TestClusterCreepStatesUsesConnectedComponents(t *testing.T) {
	states := map[creepEntityKey]creepState{
		{index: 1}: {key: creepEntityKey{index: 1}, team: 2, kind: "lane", x: 0, y: 0, alive: true},
		{index: 2}: {key: creepEntityKey{index: 2}, team: 2, kind: "lane", x: 2, y: 0, alive: true},
		{index: 3}: {key: creepEntityKey{index: 3}, team: 2, kind: "lane", x: 4, y: 0, alive: true},
	}
	clusters := clusterCreepStates(states, 2.1)
	if len(clusters) != 1 || clusters[0].CreepCount != 3 {
		t.Fatalf("clusters=%+v, want one 3-creep connected component", clusters)
	}
}

func TestClusterCreepStatesExcludesDeadAndWaiting(t *testing.T) {
	states := map[creepEntityKey]creepState{
		{index: 1}: {key: creepEntityKey{index: 1}, team: 2, kind: "lane", x: 10, y: 10, alive: true},
		{index: 2}: {key: creepEntityKey{index: 2}, team: 2, kind: "lane", x: 10, y: 10, alive: false},
		{index: 3}: {key: creepEntityKey{index: 3}, team: 2, kind: "siege", x: 10, y: 10, alive: true, waiting: true},
	}
	clusters := clusterCreepStates(states, 3)
	if len(clusters) != 1 || clusters[0].CreepCount != 1 || clusters[0].LaneCreepCount != 1 || clusters[0].SiegeCreepCount != 0 {
		t.Fatalf("clusters=%+v, want only the living spawned lane creep", clusters)
	}
}

func TestCreepCollectorAdvanceAndFinalize(t *testing.T) {
	c := newCreepCollector()
	c.started = true
	c.lastSnapshotSecond = 0
	c.validStateObserved = true
	c.states[creepEntityKey{index: 1}] = creepState{
		key: creepEntityKey{index: 1}, team: 2, kind: "lane", x: 10, y: 10, alive: true,
	}

	c.advance(2)
	out := c.finalize(3.4)
	if !out.Available {
		t.Fatal("expected available creep capability")
	}
	if len(out.Frames) != 3 {
		t.Fatalf("frames=%d, want 3", len(out.Frames))
	}
	for i, wantT := range []float64{1, 2, 3} {
		if out.Frames[i].T != wantT {
			t.Fatalf("frame %d t=%f, want %f", i, out.Frames[i].T, wantT)
		}
		if len(out.Frames[i].Clusters) != 1 || out.Frames[i].Clusters[0].CreepCount != 1 {
			t.Fatalf("frame %d clusters=%+v, want one creep", i, out.Frames[i].Clusters)
		}
	}
	if out.Method != creepClusterMethod || out.ClusterRadiusWorld != creepClusterRadiusWorld || out.ClusterRadiusTimeline != creepClusterRadiusTimeline {
		t.Fatalf("unexpected metadata: %+v", out)
	}
}

func TestCreepCollectorFinalizeEmptyFrames(t *testing.T) {
	out := newCreepCollector().finalize(0)
	if out.Available {
		t.Fatal("empty collector unexpectedly available")
	}
	if out.Frames == nil || len(out.Frames) != 0 {
		t.Fatalf("frames=%v, want explicit empty slice", out.Frames)
	}
}
