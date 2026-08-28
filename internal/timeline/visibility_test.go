package timeline

import "testing"

func TestVisibleToTeam(t *testing.T) {
	// Bit 2 = Radiant, bit 3 = Dire.
	if !visibleToTeam(1<<dotaTeamRadiant, dotaTeamRadiant) {
		t.Fatal("Radiant visibility bit was not recognized")
	}
	if visibleToTeam(1<<dotaTeamRadiant, dotaTeamDire) {
		t.Fatal("Radiant-only mask was treated as visible to Dire")
	}
	if !visibleToTeam(1<<dotaTeamDire, dotaTeamDire) {
		t.Fatal("Dire visibility bit was not recognized")
	}
	both := (1 << dotaTeamRadiant) | (1 << dotaTeamDire)
	if !visibleToTeam(both, dotaTeamRadiant) || !visibleToTeam(both, dotaTeamDire) {
		t.Fatal("combined visibility mask did not expose both teams")
	}
}

func TestMakeVisibilityEvent(t *testing.T) {
	ev := makeVisibilityEvent(42.5, 128, dotaTeamDire, 1<<dotaTeamRadiant, 100, 120)
	if ev.PlayerSlot != 128 || ev.Team != dotaTeamDire || ev.T != 42.5 {
		t.Fatalf("unexpected event identity: %#v", ev)
	}
	if !ev.VisibleToRadiant || ev.VisibleToDire {
		t.Fatalf("unexpected team visibility: %#v", ev)
	}
	if ev.X != 100 || ev.Y != 120 {
		t.Fatalf("unexpected event position: %#v", ev)
	}
}
