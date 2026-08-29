package timeline

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
)

// LaneTowerPosition is one replay-observed static lane-tower coordinate. Only
// T1-T3 lane towers are retained; T4, barracks and forts are outside this
// geometry model.
type LaneTowerPosition struct {
	Team    int     `json:"team"`
	Lane    string  `json:"lane"`
	Tier    int     `json:"tier"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	RawName string  `json:"raw_name"`
}

// ExtractLaneTowerPositions makes a narrow replay pass for static T1-T3
// coordinates. The resulting map geometry is public/static context, not
// point-in-time enemy information.
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
		if e.GetClassName() != "CDOTA_BaseNPC_Tower" {
			return nil
		}
		idx, ok := numberInt(e.Get("m_pEntity.m_nameStringTableIndex"))
		if !ok {
			return nil
		}
		name, found := p.LookupStringByIndex("EntityNames", int32(idx))
		if !found || name == "" {
			return nil
		}
		team, lane, tier, isLaneTower, malformed := parseLaneTowerEntityName(name)
		if malformed || !isLaneTower {
			return nil
		}
		x, xOK := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
		y, yOK := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
		if !xOK || !yOK {
			return nil
		}
		key := [3]int{team, laneOrder(lane), tier}
		if _, exists := observed[key]; exists {
			return nil
		}
		observed[key] = LaneTowerPosition{
			Team: team, Lane: lane, Tier: tier, X: x, Y: y, RawName: name,
		}
		return nil
	})

	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("parse replay tower positions: %w", err)
	}

	out := make([]LaneTowerPosition, 0, len(observed))
	for _, position := range observed {
		out = append(out, position)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Team != out[j].Team {
			return out[i].Team < out[j].Team
		}
		if out[i].Lane != out[j].Lane {
			return laneOrder(out[i].Lane) < laneOrder(out[j].Lane)
		}
		return out[i].Tier < out[j].Tier
	})
	return out, nil
}

func parseLaneTowerEntityName(name string) (team int, lane string, tier int, isLaneTower bool, malformed bool) {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "dota_") {
		name = "npc_" + name
	}
	return parseLaneTowerTarget(name)
}
