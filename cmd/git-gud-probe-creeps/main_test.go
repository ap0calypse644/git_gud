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

func TestAppendDistinctLimited(t *testing.T) {
	values := []string{}
	values = appendDistinctLimited(values, "a", 2)
	values = appendDistinctLimited(values, "a", 2)
	values = appendDistinctLimited(values, "b", 2)
	values = appendDistinctLimited(values, "c", 2)
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Fatalf("values=%v, want [a b]", values)
	}
}

func TestInt32Value(t *testing.T) {
	for _, value := range []any{int32(7), int(7), int64(7), uint32(7), uint64(7)} {
		got, ok := int32Value(value)
		if !ok || got != 7 {
			t.Fatalf("int32Value(%T(%v))=(%d,%v), want (7,true)", value, value, got, ok)
		}
	}
	if _, ok := int32Value("7"); ok {
		t.Fatal("int32Value(string) unexpectedly succeeded")
	}
}
