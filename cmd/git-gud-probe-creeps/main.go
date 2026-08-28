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

const maxSampleValuesPerField = 8

var creepProbeValueFields = []string{
	"m_iTeamNum",
	"m_iUnitType",
	"m_iUnitNameIndex",
	"m_pEntity.m_nameStringTableIndex",
	"CBodyComponent.m_name",
	"m_szUnitLabel",
	"m_iHealth",
	"m_iMaxHealth",
	"m_lifeState",
	"m_bIsWaitingToSpawn",
	"CBodyComponent.m_cellX",
	"CBodyComponent.m_cellY",
	"CBodyComponent.m_vecX",
	"CBodyComponent.m_vecY",
}

type classProbe struct {
	ClassName           string              `json:"class_name"`
	Created             int                 `json:"created"`
	Updated             int                 `json:"updated"`
	SampleFields        []string            `json:"sample_fields"`
	SampleValues        map[string][]string `json:"sample_values"`
	ResolvedEntityNames []string            `json:"resolved_entity_names"`
}

type output struct {
	ReplayPath string       `json:"replay_path"`
	Classes    []classProbe `json:"classes"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-probe-creeps:", err)
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

	classes := map[string]*classProbe{}
	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		className := e.GetClassName()
		if !possibleLaneCreepClass(className) {
			return nil
		}

		probe := classes[className]
		if probe == nil {
			probe = &classProbe{
				ClassName:           className,
				SampleFields:        []string{},
				SampleValues:        map[string][]string{},
				ResolvedEntityNames: []string{},
			}
			classes[className] = probe
		}
		if op.Flag(manta.EntityOpCreated) {
			probe.Created++
		}
		if op.Flag(manta.EntityOpUpdated) {
			probe.Updated++
		}

		if len(probe.SampleFields) == 0 {
			fields := e.Map()
			probe.SampleFields = make([]string, 0, len(fields))
			for key := range fields {
				probe.SampleFields = append(probe.SampleFields, key)
			}
			sort.Strings(probe.SampleFields)
		}

		for _, field := range creepProbeValueFields {
			if value := e.Get(field); value != nil {
				probe.SampleValues[field] = appendDistinctLimited(
					probe.SampleValues[field], fmt.Sprint(value), maxSampleValuesPerField,
				)
			}
		}

		// This field is the same entity-name index already used successfully by
		// the hero parser. For lane creeps it resolves to stable class-like names
		// such as npc_dota_creep_lane / npc_dota_creep_siege.
		if idx, ok := int32Value(e.Get("m_pEntity.m_nameStringTableIndex")); ok {
			if name, found := p.LookupStringByIndex("EntityNames", idx); found && name != "" {
				probe.ResolvedEntityNames = appendDistinctLimited(probe.ResolvedEntityNames, name, maxSampleValuesPerField)
			}
		}

		// m_iUnitNameIndex is retained above as an opaque raw value only. Real
		// replay validation showed that resolving it through EntityNames yields
		// unrelated path-corner / ward-spot strings, so its string-table
		// semantics are intentionally not guessed here.
		return nil
	})

	if err := p.Start(); err != nil {
		return fmt.Errorf("parse replay: %w", err)
	}

	out := output{ReplayPath: *replayPath, Classes: make([]classProbe, 0, len(classes))}
	for _, probe := range classes {
		out.Classes = append(out.Classes, *probe)
	}
	sort.Slice(out.Classes, func(i, j int) bool { return out.Classes[i].ClassName < out.Classes[j].ClassName })

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	return nil
}

func possibleLaneCreepClass(className string) bool {
	name := strings.ToLower(className)
	return strings.Contains(name, "creep") || strings.Contains(name, "lane")
}

func appendDistinctLimited(values []string, value string, limit int) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	if len(values) >= limit {
		return values
	}
	return append(values, value)
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
