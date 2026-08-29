package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dotabuff/manta"
)

const tickRate = 30.0

type transition struct {
	NetTick          uint32   `json:"net_tick"`
	GameState        *int     `json:"game_state,omitempty"`
	GameStartTime    *float64 `json:"game_start_time,omitempty"`
	GameTime         *float64 `json:"game_time,omitempty"`
	Paused           *bool    `json:"paused,omitempty"`
	PauseStartTick   *int     `json:"pause_start_tick,omitempty"`
	TotalPausedTicks *int     `json:"total_paused_ticks,omitempty"`
	DerivedGameTime  *float64 `json:"derived_game_time,omitempty"`
}

type output struct {
	ReplayPath  string       `json:"replay_path"`
	Transitions []transition `json:"transitions"`
}

type stateKey struct {
	GameState        string
	GameStartTime    string
	GameTime         string
	Paused           string
	PauseStartTick   string
	TotalPausedTicks string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-probe-clock:", err)
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

	out := output{ReplayPath: *replayPath, Transitions: []transition{}}
	var last stateKey
	seen := false

	p.OnEntity(func(e *manta.Entity, _ manta.EntityOp) error {
		if e.GetClassName() != "CDOTAGamerulesProxy" {
			return nil
		}

		state := stateKey{
			GameState:        valueString(e.Get("m_pGameRules.m_nGameState")),
			GameStartTime:    valueString(e.Get("m_pGameRules.m_flGameStartTime")),
			GameTime:         valueString(e.Get("m_pGameRules.m_fGameTime")),
			Paused:           valueString(e.Get("m_pGameRules.m_bGamePaused")),
			PauseStartTick:   valueString(e.Get("m_pGameRules.m_nPauseStartTick")),
			TotalPausedTicks: valueString(e.Get("m_pGameRules.m_nTotalPausedTicks")),
		}
		if seen && state == last {
			return nil
		}
		seen = true
		last = state

		tr := transition{NetTick: p.NetTick}
		if v, ok := intValue(e.Get("m_pGameRules.m_nGameState")); ok {
			tr.GameState = &v
		}
		if v, ok := floatValue(e.Get("m_pGameRules.m_flGameStartTime")); ok {
			tr.GameStartTime = &v
		}
		if v, ok := floatValue(e.Get("m_pGameRules.m_fGameTime")); ok {
			tr.GameTime = &v
		}
		if v, ok := boolValue(e.Get("m_pGameRules.m_bGamePaused")); ok {
			tr.Paused = &v
		}
		if v, ok := intValue(e.Get("m_pGameRules.m_nPauseStartTick")); ok {
			tr.PauseStartTick = &v
		}
		if v, ok := intValue(e.Get("m_pGameRules.m_nTotalPausedTicks")); ok {
			tr.TotalPausedTicks = &v
		}

		if tr.GameTime != nil {
			v := *tr.GameTime
			tr.DerivedGameTime = &v
		} else if tr.TotalPausedTicks != nil {
			timeTick := int(p.NetTick)
			if tr.Paused != nil && *tr.Paused && tr.PauseStartTick != nil {
				timeTick = *tr.PauseStartTick
			}
			v := float64(timeTick-*tr.TotalPausedTicks) / tickRate
			tr.DerivedGameTime = &v
		}

		out.Transitions = append(out.Transitions, tr)
		return nil
	})

	if err := p.Start(); err != nil {
		return fmt.Errorf("parse replay: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func valueString(v any) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprint(v)
}

func intValue(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	default:
		return 0, false
	}
}

func floatValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func boolValue(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}
