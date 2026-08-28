package coaching

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestCompactHeroNameTrimsReplayEntityPrefix(t *testing.T) {
	got := compactHeroName(&timeline.PlayerTimeline{HeroName: "npc_dota_hero_slark"})
	if got != "slark" {
		t.Fatalf("compact hero=%q, want slark", got)
	}
}

func TestCompactHeroNameFallsBackToHeroClass(t *testing.T) {
	got := compactHeroName(&timeline.PlayerTimeline{HeroClass: "CDOTA_Unit_Hero_Slark"})
	if got != "slark" {
		t.Fatalf("compact hero=%q, want slark", got)
	}
}
