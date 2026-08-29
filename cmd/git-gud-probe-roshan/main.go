package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/dotabuff/manta"
)

const maxSampleValuesPerField = 12

var roshanProbeFields = []string{
	"m_iTeamNum",
	"m_iHealth",
	"m_iMaxHealth",
	"m_lifeState",
	"m_bIsWaitingToSpawn",
	"m_iKillCount",
	"m_iLastKillerTeam",
	"m_hRoshan",
	"CBodyComponent.m_cellX",
	"CBodyComponent.m_cellY",
	"CBodyComponent.m_vecX",
	"CBodyComponent.m_vecY",
}

type entitySummary struct {
	Index        int32               `json:"index"`
	Serial       int32               `json:"serial"`
	ClassName    string              `json:"class_name"`
	Created      int                 `json:"created"`
	Updated      int                 `json:"updated"`
	Deleted      int                 `json:"deleted"`
	Entered      int                 `json:"entered"`
	Left         int                 `json:"left"`
	FirstNetTick uint32              `json:"first_net_tick"`
	LastNetTick  uint32              `json:"last_net_tick"`
	SampleFields []string            `json:"sample_fields"`
	SampleValues map[string][]string `json:"sample_values"`
	Transitions  []transition        `json:"transitions"`
}

type transition struct {
	NetTick        uint32 `json:"net_tick"`
	Op             string `json:"op"`
	Health         any    `json:"health,omitempty"`
	MaxHealth      any    `json:"max_health,omitempty"`
	LifeState      any    `json:"life_state,omitempty"`
	WaitingToSpawn any    `json:"waiting_to_spawn,omitempty"`
	Team           any    `json:"team,omitempty"`
	KillCount      any    `json:"kill_count,omitempty"`
	LastKillerTeam any    `json:"last_killer_team,omitempty"`
	RoshanHandle   any    `json:"roshan_handle,omitempty"`
	X              any    `json:"cell_x,omitempty"`
	Y              any    `json:"cell_y,omitempty"`
}

type output struct {
	ReplayPath string          `json:"replay_path"`
	Entities   []entitySummary `json:"entities"`
}

type stateKey struct {
	LifeState      string
	WaitingToSpawn string
	Team           string
	KillCount      string
	LastKillerTeam string
	RoshanHandle   string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-probe-roshan:", err)
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

	entities := map[string]*entitySummary{}
	lastState := map[string]stateKey{}

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		className := e.GetClassName()
		if !isRoshanLifecycleClass(className) {
			return nil
		}

		key := fmt.Sprintf("%d:%d", e.GetIndex(), e.GetSerial())
		summary := entities[key]
		if summary == nil {
			summary = &entitySummary{
				Index:        e.GetIndex(),
				Serial:       e.GetSerial(),
				ClassName:    className,
				FirstNetTick: p.NetTick,
				LastNetTick:  p.NetTick,
				SampleFields: sortedKeys(e.Map()),
				SampleValues: map[string][]string{},
				Transitions:  []transition{},
			}
			entities[key] = summary
		}
		summary.LastNetTick = p.NetTick

		if op.Flag(manta.EntityOpCreated) {
			summary.Created++
		}
		if op.Flag(manta.EntityOpUpdated) {
			summary.Updated++
		}
		if op.Flag(manta.EntityOpDeleted) {
			summary.Deleted++
		}
		if op.Flag(manta.EntityOpEntered) {
			summary.Entered++
		}
		if op.Flag(manta.EntityOpLeft) {
			summary.Left++
		}

		for _, field := range roshanProbeFields {
			if value := e.Get(field); value != nil {
				summary.SampleValues[field] = appendDistinctLimited(summary.SampleValues[field], fmt.Sprint(value), maxSampleValuesPerField)
			}
		}

		state := stateKey{
			LifeState:      valueString(e.Get("m_lifeState")),
			WaitingToSpawn: valueString(e.Get("m_bIsWaitingToSpawn")),
			Team:           valueString(e.Get("m_iTeamNum")),
			KillCount:      valueString(e.Get("m_iKillCount")),
			LastKillerTeam: valueString(e.Get("m_iLastKillerTeam")),
			RoshanHandle:   valueString(e.Get("m_hRoshan")),
		}
		previous, seen := lastState[key]
		stateChanged := !seen || state != previous
		boundaryOp := op.Flag(manta.EntityOpCreated) || op.Flag(manta.EntityOpDeleted) || op.Flag(manta.EntityOpEntered) || op.Flag(manta.EntityOpLeft)
		if stateChanged || boundaryOp {
			summary.Transitions = append(summary.Transitions, transition{
				NetTick:        p.NetTick,
				Op:             op.String(),
				Health:         e.Get("m_iHealth"),
				MaxHealth:      e.Get("m_iMaxHealth"),
				LifeState:      e.Get("m_lifeState"),
				WaitingToSpawn: e.Get("m_bIsWaitingToSpawn"),
				Team:           e.Get("m_iTeamNum"),
				KillCount:      e.Get("m_iKillCount"),
				LastKillerTeam: e.Get("m_iLastKillerTeam"),
				RoshanHandle:   e.Get("m_hRoshan"),
				X:              e.Get("CBodyComponent.m_cellX"),
				Y:              e.Get("CBodyComponent.m_cellY"),
			})
			lastState[key] = state
		}
		return nil
	})

	if err := p.Start(); err != nil {
		return fmt.Errorf("parse replay: %w", err)
	}

	out := output{ReplayPath: *replayPath, Entities: make([]entitySummary, 0, len(entities))}
	for _, summary := range entities {
		out.Entities = append(out.Entities, *summary)
	}
	sort.Slice(out.Entities, func(i, j int) bool {
		if out.Entities[i].FirstNetTick != out.Entities[j].FirstNetTick {
			return out.Entities[i].FirstNetTick < out.Entities[j].FirstNetTick
		}
		if out.Entities[i].Index != out.Entities[j].Index {
			return out.Entities[i].Index < out.Entities[j].Index
		}
		return out.Entities[i].Serial < out.Entities[j].Serial
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	return nil
}

func isRoshanLifecycleClass(className string) bool {
	return className == "CDOTA_Unit_Roshan" || className == "CDOTA_RoshanSpawner"
}

func sortedKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func valueString(value any) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprint(value)
}
