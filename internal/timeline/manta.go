package timeline

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"
)

const (
	tickRate      = 30.0
	steamID64Base = uint64(76561197960265728)
)

type ParseOptions struct {
	MatchID          int64
	AccountID        uint32
	TargetPlayerSlot int // set to -1 when OpenDota cannot provide it
}

type rawDeath struct {
	timestamp       float64
	attacker        string
	victim          string
	inflictor       string
	assistPlayerIDs []int32
}

// Parse consumes a decompressed Source 2 replay and returns deterministic,
// roughly one-second hero snapshots plus exact replay-derived events. The
// parser records all ten heroes because later coaching needs the map context
// around the configured player, not only the player's own coordinates.
func Parse(r io.Reader, opts ParseOptions) (MatchTimeline, error) {
	p, err := manta.NewStreamParser(r)
	if err != nil {
		return MatchTimeline{}, fmt.Errorf("create manta parser: %w", err)
	}

	out := MatchTimeline{
		MatchID:          opts.MatchID,
		AccountID:        opts.AccountID,
		TargetPlayerSlot: opts.TargetPlayerSlot,
		Players:          make(map[string]*PlayerTimeline),
	}

	lastSampleSecond := make(map[int]int)
	for slot := 0; slot < 5; slot++ {
		lastSampleSecond[slot] = math.MinInt
		lastSampleSecond[128+slot] = math.MinInt
	}

	var gameStartTime float64
	var gameStartSet bool
	var gameEndTime float64
	var gameEndSet bool
	var deaths []rawDeath

	events := newEventCollector(p)
	visibility := newVisibilityCollector()
	creeps := newCreepCollector()

	combatLogName := func(index uint32) string {
		name, _ := p.LookupStringByIndex("CombatLogNames", int32(index))
		return strings.TrimPrefix(name, "item_")
	}

	p.Callbacks.OnCMsgDOTACombatLogEntry(func(m *dota.CMsgDOTACombatLogEntry) error {
		if m.GetType() != dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DEATH {
			return nil
		}
		if !m.GetIsTargetHero() || m.GetIsTargetIllusion() {
			return nil
		}
		deaths = append(deaths, rawDeath{
			timestamp:       float64(m.GetTimestamp()),
			attacker:        combatLogName(m.GetAttackerName()),
			victim:          combatLogName(m.GetTargetName()),
			inflictor:       combatLogName(m.GetInflictorName()),
			assistPlayerIDs: m.GetAssistPlayers(),
		})
		return nil
	})

	targetSteamID := steamID64Base + uint64(opts.AccountID)

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		className := e.GetClassName()

		if className == "CDOTAGamerulesProxy" {
			if state, ok := numberInt(e.Get("m_pGameRules.m_nGameState")); ok {
				if state >= 5 && !gameStartSet {
					if start, ok := numberFloat(e.Get("m_pGameRules.m_flGameStartTime")); ok && start > 0 {
						gameStartTime = start
						gameStartSet = true
					}
				}
				if state >= 6 && gameStartSet && !gameEndSet {
					gameEndTime = float64(p.NetTick) / tickRate
					gameEndSet = true
				}
			}
			return nil
		}

		// CDOTA_PlayerResource uses a conventional global 0-9 player index.
		// Capture its team mapping even when OpenDota already supplied the
		// target slot because buyback and assist records use this numbering.
		if className == "CDOTA_PlayerResource" {
			events.observePlayerResource(e)
			if out.TargetPlayerSlot < 0 {
				for resourcePlayerID := 0; resourcePlayerID < 10; resourcePlayerID++ {
					steamID, ok := playerSteamID(e, resourcePlayerID)
					if !ok || steamID != targetSteamID {
						continue
					}
					team, ok := playerTeam(e, resourcePlayerID)
					if !ok {
						if resourcePlayerID < 5 {
							team = 2
						} else {
							team = 3
						}
					}
					if slot, ok := resourcePlayerSlot(resourcePlayerID, team); ok {
						out.TargetPlayerSlot = slot
					}
					break
				}
			}
			return nil
		}

		// M12 derives compact same-team creep spatial clusters directly during
		// replay parsing. M13 reuses the same validated creep callbacks while
		// retaining observed activation cohorts for lane-wave identity.
		if gameStartSet && !gameEndSet {
			matchTime := float64(p.NetTick)/tickRate - gameStartTime
			if matchTime >= 0 {
				creeps.observe(e, op, matchTime)
			}
		}

		if !strings.HasPrefix(className, "CDOTA_Unit_Hero_") {
			return nil
		}
		if !op.Flag(manta.EntityOpUpdated) && !op.Flag(manta.EntityOpCreated) {
			return nil
		}

		rawPlayerID, ok := numberInt(e.Get("m_iPlayerID"))
		if !ok {
			return nil
		}
		team, ok := numberInt(e.Get("m_iTeamNum"))
		if !ok {
			return nil
		}
		slot, ok := heroPlayerSlot(rawPlayerID, team)
		if !ok {
			return nil
		}

		// Resolve the exact EntityNames string used by the combat log. This is
		// deliberately done before the in-game sampling gate so pre-game hero
		// entities can establish combat-log attribution too.
		events.observeHero(e, slot)

		if !gameStartSet || gameEndSet {
			return nil
		}

		matchTime := float64(p.NetTick)/tickRate - gameStartTime
		if matchTime < 0 {
			return nil
		}

		// Visibility changes are exact replay facts and must be observed before
		// the roughly-1Hz hero snapshot throttle. Modern replay builds may omit
		// this field; the collector records that capability explicitly.
		visibility.observe(&out, e, slot, team, matchTime)

		second := int(math.Floor(matchTime))
		if lastSampleSecond[slot] == second {
			return nil
		}

		x, xOK := cellPosition(e, "CBodyComponent.m_cellX", "CBodyComponent.m_vecX")
		y, yOK := cellPosition(e, "CBodyComponent.m_cellY", "CBodyComponent.m_vecY")
		if !xOK || !yOK {
			return nil
		}

		pt := HeroSample{T: matchTime, X: x, Y: y}
		if v, ok := numberInt(e.Get("m_iHealth")); ok {
			pt.HP = int32(v)
		}
		if v, ok := numberInt(e.Get("m_iMaxHealth")); ok {
			pt.MaxHP = int32(v)
		}
		if v, ok := numberFloat(e.Get("m_flMana")); ok {
			pt.Mana = float32(v)
		}
		if v, ok := numberFloat(e.Get("m_flMaxMana")); ok {
			pt.MaxMana = float32(v)
		}
		if v, ok := numberInt(e.Get("m_iCurrentLevel")); ok {
			pt.Level = int32(v)
		}
		pt.Alive = pt.HP > 0
		if lifeState, ok := numberInt(e.Get("m_lifeState")); ok {
			pt.Alive = lifeState == 0
		}

		key := fmt.Sprintf("%d", slot)
		player := out.Players[key]
		if player == nil {
			player = &PlayerTimeline{
				PlayerSlot: slot,
				PlayerID:   rawPlayerID,
				Team:       team,
				HeroClass:  className,
			}
			out.Players[key] = player
		}
		player.HeroClass = className
		player.Samples = append(player.Samples, pt)
		lastSampleSecond[slot] = second
		return nil
	})

	if err := p.Start(); err != nil {
		return MatchTimeline{}, fmt.Errorf("parse replay: %w", err)
	}
	out.GameBuild = p.GameBuild

	if !gameStartSet {
		return MatchTimeline{}, fmt.Errorf("replay did not expose game start time")
	}
	if out.TargetPlayerSlot < 0 {
		return MatchTimeline{}, fmt.Errorf("account %d was not resolved to a replay player slot", opts.AccountID)
	}
	if target := out.Players[fmt.Sprintf("%d", out.TargetPlayerSlot)]; target == nil || len(target.Samples) == 0 {
		return MatchTimeline{}, fmt.Errorf("target player slot %d has no hero samples", out.TargetPlayerSlot)
	}

	for _, d := range deaths {
		t := d.timestamp - gameStartTime
		death := DeathEvent{
			T:         t,
			Attacker:  d.attacker,
			Victim:    d.victim,
			Inflictor: d.inflictor,
		}
		if slot, ok := events.heroSlot(d.attacker); ok {
			death.AttackerSlot = intPtr(slot)
		}
		if slot, ok := events.heroSlot(d.victim); ok {
			death.VictimSlot = intPtr(slot)
		}
		for _, playerID := range d.assistPlayerIDs {
			if slot, ok := events.resourceSlot(int(playerID)); ok {
				death.AssistSlots = append(death.AssistSlots, slot)
			}
		}
		out.Deaths = append(out.Deaths, death)
		if t > out.DurationSeconds {
			out.DurationSeconds = t
		}
	}

	if gameEndSet {
		out.DurationSeconds = gameEndTime - gameStartTime
	} else {
		for _, player := range out.Players {
			if n := len(player.Samples); n > 0 && player.Samples[n-1].T > out.DurationSeconds {
				out.DurationSeconds = player.Samples[n-1].T
			}
		}
	}

	out.CreepClusters = creeps.finalize(out.DurationSeconds)
	out.LaneWaves = creeps.finalizeLaneWaves(out.DurationSeconds)
	events.apply(&out, gameStartTime)
	out.Fights = DeriveFightWindows(&out)

	return out, nil
}

func intPtr(v int) *int { return &v }

func playerSteamID(e *manta.Entity, playerID int) (uint64, bool) {
	fields := []string{
		fmt.Sprintf("m_vecPlayerData.%04d.m_iPlayerSteamID", playerID),
		fmt.Sprintf("m_iPlayerSteamIDs.%04d", playerID),
	}
	for _, field := range fields {
		if v, ok := numberUint(e.Get(field)); ok {
			return v, true
		}
	}
	return 0, false
}

func playerTeam(e *manta.Entity, playerID int) (int, bool) {
	fields := []string{
		fmt.Sprintf("m_vecPlayerData.%04d.m_iPlayerTeam", playerID),
		fmt.Sprintf("m_iPlayerTeams.%04d", playerID),
	}
	for _, field := range fields {
		if v, ok := numberInt(e.Get(field)); ok {
			return v, true
		}
	}
	return 0, false
}

// resourcePlayerSlot maps CDOTA_PlayerResource's global 0-9 player index to
// OpenDota/Dota's player_slot convention.
func resourcePlayerSlot(playerID, team int) (int, bool) {
	switch team {
	case 2:
		if playerID >= 0 && playerID < 5 {
			return playerID, true
		}
	case 3:
		if playerID >= 5 && playerID < 10 {
			return 128 + playerID - 5, true
		}
		// Some older/demo variants expose a team-relative PlayerResource ID.
		if playerID >= 0 && playerID < 5 {
			return 128 + playerID, true
		}
	}
	return 0, false
}

// heroPlayerSlot maps a hero entity's raw m_iPlayerID/m_iTeamNum pair to
// OpenDota/Dota's player_slot convention. Source 2 hero entities use even
// player IDs: 0,2,4,6,8 for Radiant and 10,12,14,16,18 for Dire.
func heroPlayerSlot(playerID, team int) (int, bool) {
	if playerID%2 != 0 {
		return 0, false
	}
	switch team {
	case 2:
		if playerID < 0 || playerID > 8 {
			return 0, false
		}
		return playerID / 2, true
	case 3:
		if playerID < 10 || playerID > 18 {
			return 0, false
		}
		return 128 + (playerID-10)/2, true
	}
	return 0, false
}

func cellPosition(e *manta.Entity, cellField, vectorField string) (float64, bool) {
	cell, ok := numberFloat(e.Get(cellField))
	if !ok {
		return 0, false
	}
	vec, ok := numberFloat(e.Get(vectorField))
	if !ok {
		return 0, false
	}
	return cell + vec/128.0, true
}

func numberInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		if n > uint64(^uint(0)>>1) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func numberUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint8:
		return uint64(n), true
	case uint16:
		return uint64(n), true
	case uint32:
		return uint64(n), true
	case uint64:
		return n, true
	case int:
		if n >= 0 {
			return uint64(n), true
		}
	case int32:
		if n >= 0 {
			return uint64(n), true
		}
	case int64:
		if n >= 0 {
			return uint64(n), true
		}
	}
	return 0, false
}

func numberFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
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
