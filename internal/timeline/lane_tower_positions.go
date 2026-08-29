package timeline

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
)

// LaneTowerPosition is one replay-observed static lane-structure coordinate.
// T1-T3 lane towers use their normal lane/tier values. The two public-map Fort
// entities are also retained as Lane="base", Tier=0 so later objective logic
// can use exact Ancient-centered backdoor geometry without a second replay pass.
type LaneTowerPosition struct {
	Team    int     `json:"team"`
	Lane    string  `json:"lane"`
	Tier    int     `json:"tier"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	RawName string  `json:"raw_name"`
}

// observeLaneTowerPosition records one static T1-T3 lane tower or Fort when the
// replay entity exposes a resolvable EntityNames entry and position. The
// callback is shared by the main parser and standalone structure probe so
// production parsing does not need a second replay pass.
func observeLaneTowerPosition(
	e *manta.Entity,
	lookupEntityName func(int32) (string, bool),
	observed map[[3]int]LaneTowerPosition,
) {
	className := e.GetClassName()
	if className != "CDOTA_BaseNPC_Tower" && className != "CDOTA_BaseNPC_Fort" {
		return
	}
	idx, ok := numberInt(e.Get("m_pEntity.m_nameStringTableIndex"))
	if !ok {
		return
	}
	name, found := lookupEntityName(int32(idx))
	if !found || name == "" {
		return
	}

	var team, tier int
	var lane string
	if className == "CDOTA_BaseNPC_Fort" {
		team, ok = parseFortEntityName(name)
		if !ok {
			return
		}
		lane = "base"
		tier = 0
	} else {
		var isLaneTower, malformed bool
		team, lane, tier, isLaneTower, malformed = parseLaneTowerEntityName(name)
		if malformed || !isLaneTower {
			return
		}
	}

	x, xOK := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
	y, yOK := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
	if !xOK || !yOK {
		return
	}
	laneKey := laneOrder(lane)
	if lane == "base" {
		laneKey = -1
	}
	key := [3]int{team, laneKey, tier}
	if _, exists := observed[key]; exists {
		return
	}
	observed[key] = LaneTowerPosition{
		Team: team, Lane: lane, Tier: tier, X: x, Y: y, RawName: name,
	}
}

func finalizeLaneTowerPositions(observed map[[3]int]LaneTowerPosition) []LaneTowerPosition {
	out := make([]LaneTowerPosition, 0, len(observed))
	for _, position := range observed {
		out = append(out, position)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Team != out[j].Team {
			return out[i].Team < out[j].Team
		}
		if out[i].Lane == "base" || out[j].Lane == "base" {
			if out[i].Lane != out[j].Lane {
				return out[i].Lane != "base"
			}
			return out[i].Tier < out[j].Tier
		}
		if out[i].Lane != out[j].Lane {
			return laneOrder(out[i].Lane) < laneOrder(out[j].Lane)
		}
		return out[i].Tier < out[j].Tier
	})
	return out
}

// ExtractLaneTowerPositions is retained for the standalone probe/debug path.
// Production timeline building observes the same entities during Parse instead
// of reparsing the replay solely for static structure coordinates. The returned
// slice also includes the two Fort entries used by objective backdoor logic.
func ExtractLaneTowerPositions(replayPath string) ([]LaneTowerPosition, error) {
	f, err := os.Open(replayPath)
	if err != nil {
		return nil, fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		return nil, fmt.Errorf("create manta parser: %w", err)
	}

	observed := make(map[[3]int]LaneTowerPosition)
	p.OnEntity(func(e *manta.Entity, _ manta.EntityOp) error {
		observeLaneTowerPosition(e, func(index int32) (string, bool) {
			return p.LookupStringByIndex("EntityNames", index)
		}, observed)
		return nil
	})

	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("parse replay tower positions: %w", err)
	}

	return finalizeLaneTowerPositions(observed), nil
}

func parseLaneTowerEntityName(name string) (team int, lane string, tier int, isLaneTower bool, malformed bool) {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "dota_") {
		name = "npc_" + name
	}
	return parseLaneTowerTarget(name)
}

func parseFortEntityName(name string) (int, bool) {
	switch strings.TrimSpace(name) {
	case "dota_goodguys_fort", "npc_dota_goodguys_fort":
		return 2, true
	case "dota_badguys_fort", "npc_dota_badguys_fort":
		return 3, true
	default:
		return 0, false
	}
}
