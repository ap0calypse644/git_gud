package timeline

import "testing"

func TestWardKind(t *testing.T) {
	cases := map[string]string{
		"CDOTA_NPC_Observer_Ward":           "observer",
		"DT_DOTA_NPC_Observer_Ward":          "observer",
		"CDOTA_NPC_Observer_Ward_TrueSight": "sentry",
		"DT_DOTA_NPC_Observer_Ward_TrueSight": "sentry",
	}
	for className, want := range cases {
		got, ok := wardKind(className)
		if !ok || got != want {
			t.Fatalf("wardKind(%q) = %q,%v; want %q,true", className, got, ok, want)
		}
	}
	if _, ok := wardKind("CDOTA_Unit_Hero_Slark"); ok {
		t.Fatal("hero class was treated as a ward")
	}
}

func TestWardCollectorApplyNormalizesTimes(t *testing.T) {
	owner := 4
	c := &wardCollector{intervals: []rawWardInterval{
		{
			key:              wardEntityKey{index: 77, serial: 9},
			kind:             "observer",
			team:             dotaTeamRadiant,
			ownerRawPlayerID: &owner,
			x:                101.5,
			y:                122.25,
			dayVisionRange:   1600,
			nightVisionRange: 1600,
			startAbs:         105,
			endAbs:           165,
			endReason:        "life_state_ended",
			closed:           true,
		},
	}}

	out := MatchTimeline{}
	c.apply(&out, 100, 300)
	if len(out.VisionSources.Wards) != 1 {
		t.Fatalf("got %d wards, want 1", len(out.VisionSources.Wards))
	}
	got := out.VisionSources.Wards[0]
	if got.StartT != 5 || got.EndT != 65 {
		t.Fatalf("normalized lifetime %.1f..%.1f, want 5..65", got.StartT, got.EndT)
	}
	if got.EntityIndex != 77 || got.EntitySerial != 9 || got.Team != dotaTeamRadiant {
		t.Fatalf("unexpected ward identity: %#v", got)
	}
	if got.OwnerRawPlayerID == nil || *got.OwnerRawPlayerID != owner {
		t.Fatalf("owner = %#v, want %d", got.OwnerRawPlayerID, owner)
	}
	if got.DayVisionRange != 1600 || got.NightVisionRange != 1600 {
		t.Fatalf("vision ranges = %.0f/%.0f", got.DayVisionRange, got.NightVisionRange)
	}
}

func TestWardCollectorApplyClampsPregameAndGameEnd(t *testing.T) {
	c := &wardCollector{intervals: []rawWardInterval{
		{
			key:      wardEntityKey{index: 1, serial: 1},
			kind:     "observer",
			team:     dotaTeamDire,
			startAbs: 90,
			closed:   false,
		},
	}}

	out := MatchTimeline{}
	c.apply(&out, 100, 200)
	if len(out.VisionSources.Wards) != 1 {
		t.Fatalf("got %d wards, want 1", len(out.VisionSources.Wards))
	}
	got := out.VisionSources.Wards[0]
	if got.StartT != 0 || got.EndT != 200 || got.EndReason != "game_end" {
		t.Fatalf("pregame/game-end clamp = %#v", got)
	}
}

func TestWardCollectorApplyEmitsEmptySlice(t *testing.T) {
	c := &wardCollector{}
	out := MatchTimeline{}
	c.apply(&out, 100, 200)
	if out.VisionSources.Wards == nil {
		t.Fatal("empty ward timeline should serialize as [] rather than null")
	}
}
