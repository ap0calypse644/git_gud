package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
)

type structureObservation struct {
	Name      string  `json:"name,omitempty"`
	ClassName string  `json:"class_name"`
	Team      int     `json:"team,omitempty"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	HasXY     bool    `json:"has_xy"`
	Seen      int     `json:"seen"`
}

type output struct {
	ReplayPath   string                 `json:"replay_path"`
	Observations []structureObservation `json:"observations"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-probe-structures:", err)
		os.Exit(1)
	}
}

func run() error {
	replayPath := flag.String("replay", "", "path to a decompressed Source 2 .dem replay")
	flag.Parse()
	if *replayPath == "" {
		return fmt.Errorf("-replay is required")
	}

	f, err := os.Open(*replayPath)
	if err != nil {
		return fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		return fmt.Errorf("create manta parser: %w", err)
	}

	observed := map[string]*structureObservation{}
	p.OnEntity(func(e *manta.Entity, _ manta.EntityOp) error {
		className := e.GetClassName()
		name := ""
		if idx, ok := int32Value(e.Get("m_pEntity.m_nameStringTableIndex")); ok {
			if resolved, found := p.LookupStringByIndex("EntityNames", idx); found {
				name = resolved
			}
		}
		if !possibleStructure(className, name) {
			return nil
		}

		key := name
		if key == "" {
			key = "class:" + className
		}
		obs := observed[key]
		if obs == nil {
			obs = &structureObservation{Name: name, ClassName: className}
			observed[key] = obs
		}
		obs.Seen++
		if team, ok := intValue(e.Get("m_iTeamNum")); ok {
			obs.Team = team
		}
		if x, y, ok := entityXY(e); ok {
			obs.X = x
			obs.Y = y
			obs.HasXY = true
		}
		return nil
	})

	if err := p.Start(); err != nil {
		return fmt.Errorf("parse replay: %w", err)
	}

	out := output{ReplayPath: *replayPath, Observations: make([]structureObservation, 0, len(observed))}
	for _, obs := range observed {
		out.Observations = append(out.Observations, *obs)
	}
	sort.Slice(out.Observations, func(i, j int) bool {
		if out.Observations[i].Name != out.Observations[j].Name {
			return out.Observations[i].Name < out.Observations[j].Name
		}
		return out.Observations[i].ClassName < out.Observations[j].ClassName
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	return nil
}

func possibleStructure(className, entityName string) bool {
	value := strings.ToLower(className + " " + entityName)
	for _, needle := range []string{"tower", "barracks", "rax", "fort"} {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func entityXY(e *manta.Entity) (float64, float64, bool) {
	cellX, okX := numberValue(e.Get("CBodyComponent.m_cellX"))
	cellY, okY := numberValue(e.Get("CBodyComponent.m_cellY"))
	vecX, okVX := numberValue(e.Get("CBodyComponent.m_vecX"))
	vecY, okVY := numberValue(e.Get("CBodyComponent.m_vecY"))
	if !okX || !okY || !okVX || !okVY {
		return 0, 0, false
	}
	return cellX + vecX/128.0, cellY + vecY/128.0, true
}

func int32Value(value any) (int32, bool) {
	switch v := value.(type) {
	case int32:
		return v, true
	case int:
		return int32(v), true
	case int64:
		return int32(v), true
	case uint32:
		return int32(v), true
	case uint64:
		return int32(v), true
	default:
		return 0, false
	}
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	default:
		return 0, false
	}
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
