package main

import "testing"

func TestPossibleLaneCreepClass(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"CDOTA_BaseNPC_Creep_Lane", true},
		{"CDOTA_BaseNPC_Creep_Siege", true},
		{"SomeLaneEntity", true},
		{"CDOTA_Unit_Hero_Slark", false},
		{"CDOTA_NPC_Observer_Ward", false},
	}
	for _, tt := range tests {
		if got := possibleLaneCreepClass(tt.name); got != tt.want {
			t.Fatalf("possibleLaneCreepClass(%q)=%v, want %v", tt.name, got, tt.want)
		}
	}
}
