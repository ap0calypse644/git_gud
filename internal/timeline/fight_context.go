package timeline

import (
	"math"
	"sort"
)

// TargetFightContext is deterministic target-centric context for one final
// consolidated fight. It is evidence only: it does not say the player should
// have joined, should have avoided the fight, or played it well.
type TargetFightContext struct {
	StartT                 float64 `json:"start_t"`
	EndT                   float64 `json:"end_t"`
	ObservedStartT         float64 `json:"observed_start_t"`
	ObservedEndT           float64 `json:"observed_end_t"`
	ObservedTimingAvailable bool   `json:"observed_timing_available"`

	CenterX        float64 `json:"center_x,omitempty"`
	CenterY        float64 `json:"center_y,omitempty"`
	Participants   []int   `json:"participants"`
	Deaths         int     `json:"deaths"`
	HeroDamage     int64   `json:"hero_damage"`
	TargetInvolved bool    `json:"target_involved"`

	TargetAtStart       FightTargetState     `json:"target_at_start"`
	TeammatesAtStart    []FightTeammateState `json:"teammates_at_start"`
	NearbyAlliesAtStart []NearbyAlly          `json:"nearby_allies_at_start"`
	EnemyKnowledgeAtStart []EnemyKnowledgeState `json:"enemy_knowledge_at_start"`

	TargetFirstInvolvementT      *float64 `json:"target_first_involvement_t,omitempty"`
	TargetFirstInvolvementSource string   `json:"target_first_involvement_source,omitempty"`
	SecondsToFirstInvolvement    *float64 `json:"seconds_to_first_involvement,omitempty"`
	TargetFirstDamageDealtT      *float64 `json:"target_first_damage_dealt_t,omitempty"`
	TargetFirstDamageReceivedT   *float64 `json:"target_first_damage_received_t,omitempty"`
	TargetFirstAbilityT          *float64 `json:"target_first_ability_t,omitempty"`
	TargetDeathT                 *float64 `json:"target_death_t,omitempty"`

	TargetDamageDealt    int64 `json:"target_damage_dealt"`
	TargetDamageReceived int64 `json:"target_damage_received"`
	TargetAbilityCount   int   `json:"target_ability_count"`

	AlliedDeathsBeforeTargetInvolvement []int `json:"allied_deaths_before_target_involvement"`
}

// FightTargetState is the freshest target hero sample at or before fight
// start. A stale/missing sample remains explicit rather than becoming a zero
// coordinate or fabricated alive state.
type FightTargetState struct {
	SampleAvailable       bool    `json:"sample_available"`
	SampleT               float64 `json:"sample_t,omitempty"`
	Alive                 bool    `json:"alive,omitempty"`
	X                     float64 `json:"x,omitempty"`
	Y                     float64 `json:"y,omitempty"`
	DistanceToFightCenter float64 `json:"distance_to_fight_center,omitempty"`
	HP                    int32   `json:"hp,omitempty"`
	MaxHP                 int32   `json:"max_hp,omitempty"`
	Mana                  float32 `json:"mana,omitempty"`
	MaxMana               float32 `json:"max_mana,omitempty"`
	Level                 int32   `json:"level,omitempty"`
}

// FightTeammateState records one allied primary hero at fight start. Position
// and life state are replay facts for an ally, not enemy knowledge.
type FightTeammateState struct {
	PlayerSlot                int     `json:"player_slot"`
	SampleAvailable           bool    `json:"sample_available"`
	SampleT                   float64 `json:"sample_t,omitempty"`
	Alive                     bool    `json:"alive,omitempty"`
	X                         float64 `json:"x,omitempty"`
	Y                         float64 `json:"y,omitempty"`
	DistanceToFightCenter     float64 `json:"distance_to_fight_center,omitempty"`
	DistanceToTargetAvailable bool    `json:"distance_to_target_available"`
	DistanceToTarget          float64 `json:"distance_to_target,omitempty"`
}

// DeriveTargetFightContexts builds participation evidence for every final
// consolidated fight, including fights the target never joined. That shared
// representation is required before later bad-join or missed-fight judgments
// can be made safely.
func DeriveTargetFightContexts(tl *MatchTimeline) []TargetFightContext {
	if tl == nil {
		return []TargetFightContext{}
	}
	target := tl.Players[slotKey(tl.TargetPlayerSlot)]
	if target == nil {
		return []TargetFightContext{}
	}

	out := make([]TargetFightContext, 0, len(tl.Fights))
	for _, fight := range tl.Fights {
		ctx := TargetFightContext{
			StartT:                  fight.StartT,
			EndT:                    fight.EndT,
			ObservedStartT:          fight.ObservedStartT,
			ObservedEndT:            fight.ObservedEndT,
			ObservedTimingAvailable: observedFightTimingAvailable(fight),
			CenterX:                 fight.CenterX,
			CenterY:                 fight.CenterY,
			Participants:            append([]int(nil), fight.Participants...),
			Deaths:                  fight.Deaths,
			HeroDamage:              fight.HeroDamage,
			TargetInvolved:          fightContainsSlot(fight, tl.TargetPlayerSlot),
			TeammatesAtStart:        []FightTeammateState{},
			NearbyAlliesAtStart:     []NearbyAlly{},
			EnemyKnowledgeAtStart:   []EnemyKnowledgeState{},
			AlliedDeathsBeforeTargetInvolvement: []int{},
		}

		startT := fight.StartT
		if ctx.ObservedTimingAvailable {
			startT = fight.ObservedStartT
		}
		ctx.TargetAtStart = fightTargetStateAt(target, startT, fight.CenterX, fight.CenterY)
		ctx.TeammatesAtStart = fightTeammatesAtStart(tl, target.Team, tl.TargetPlayerSlot, startT, fight.CenterX, fight.CenterY, ctx.TargetAtStart)
		if ctx.TargetAtStart.SampleAvailable {
			ctx.NearbyAlliesAtStart = nearbyAlliesAt(tl, target.Team, tl.TargetPlayerSlot, startT, ctx.TargetAtStart.X, ctx.TargetAtStart.Y)
		}
		ctx.EnemyKnowledgeAtStart = enemyKnowledgeAt(tl, target.Team, startT)

		if ctx.ObservedTimingAvailable {
			populateTargetFightParticipation(tl, target.Team, &ctx)
		}
		out = append(out, ctx)
	}

	sort.SliceStable(out, func(i, j int) bool {
		ai := out[i].StartT
		aj := out[j].StartT
		if out[i].ObservedTimingAvailable {
			ai = out[i].ObservedStartT
		}
		if out[j].ObservedTimingAvailable {
			aj = out[j].ObservedStartT
		}
		return ai < aj
	})
	return out
}

func observedFightTimingAvailable(fight FightWindow) bool {
	if fight.ObservedEndT < fight.ObservedStartT {
		return false
	}
	return fight.ObservedStartT != 0 || fight.ObservedEndT != 0
}

func fightTargetStateAt(player *PlayerTimeline, t, centerX, centerY float64) FightTargetState {
	sample, ok := heroSampleAtOrBefore(player, t)
	if !ok {
		return FightTargetState{}
	}
	return FightTargetState{
		SampleAvailable:       true,
		SampleT:               sample.T,
		Alive:                 sample.Alive,
		X:                     sample.X,
		Y:                     sample.Y,
		DistanceToFightCenter: math.Hypot(sample.X-centerX, sample.Y-centerY),
		HP:                    sample.HP,
		MaxHP:                 sample.MaxHP,
		Mana:                  sample.Mana,
		MaxMana:               sample.MaxMana,
		Level:                 sample.Level,
	}
}

func fightTeammatesAtStart(tl *MatchTimeline, team, targetSlot int, t, centerX, centerY float64, target FightTargetState) []FightTeammateState {
	out := make([]FightTeammateState, 0, 4)
	for _, player := range tl.Players {
		if player == nil || player.Team != team || player.PlayerSlot == targetSlot {
			continue
		}
		state := FightTeammateState{PlayerSlot: player.PlayerSlot}
		if sample, ok := heroSampleAtOrBefore(player, t); ok {
			state.SampleAvailable = true
			state.SampleT = sample.T
			state.Alive = sample.Alive
			state.X = sample.X
			state.Y = sample.Y
			state.DistanceToFightCenter = math.Hypot(sample.X-centerX, sample.Y-centerY)
			if target.SampleAvailable {
				state.DistanceToTargetAvailable = true
				state.DistanceToTarget = math.Hypot(sample.X-target.X, sample.Y-target.Y)
			}
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlayerSlot < out[j].PlayerSlot })
	return out
}

func populateTargetFightParticipation(tl *MatchTimeline, targetTeam int, ctx *TargetFightContext) {
	if tl == nil || ctx == nil {
		return
	}
	startT := ctx.ObservedStartT
	endT := ctx.ObservedEndT
	targetSlot := tl.TargetPlayerSlot

	var firstT *float64
	firstSource := ""
	firstPriority := 100
	considerFirst := func(t float64, source string, priority int) {
		if t < startT || t > endT {
			return
		}
		if firstT == nil || t < *firstT || (t == *firstT && priority < firstPriority) {
			v := t
			firstT = &v
			firstSource = source
			firstPriority = priority
		}
	}

	for _, event := range tl.Damage {
		if event.T < startT || event.T > endT || event.Value <= 0 {
			continue
		}
		value := int64(event.Value)
		if event.AttackerSlot == targetSlot {
			ctx.TargetDamageDealt += value
			if ctx.TargetFirstDamageDealtT == nil {
				ctx.TargetFirstDamageDealtT = float64Ptr(event.T)
			}
			considerFirst(event.T, "damage_dealt", 0)
		}
		if event.VictimSlot == targetSlot {
			ctx.TargetDamageReceived += value
			if ctx.TargetFirstDamageReceivedT == nil {
				ctx.TargetFirstDamageReceivedT = float64Ptr(event.T)
			}
			considerFirst(event.T, "damage_received", 0)
		}
	}

	for _, event := range tl.Deaths {
		if event.T < startT || event.T > endT {
			continue
		}
		if event.VictimSlot != nil && *event.VictimSlot == targetSlot {
			if ctx.TargetDeathT == nil {
				ctx.TargetDeathT = float64Ptr(event.T)
			}
			considerFirst(event.T, "death", 1)
		}
		if event.AttackerSlot != nil && *event.AttackerSlot == targetSlot {
			considerFirst(event.T, "kill", 1)
		}
		if intSliceContains(event.AssistSlots, targetSlot) {
			considerFirst(event.T, "assist", 2)
		}
	}

	for _, event := range tl.Abilities {
		if event.T < startT || event.T > endT || event.PlayerSlot != targetSlot {
			continue
		}
		ctx.TargetAbilityCount++
		if ctx.TargetFirstAbilityT == nil {
			ctx.TargetFirstAbilityT = float64Ptr(event.T)
		}
		considerFirst(event.T, "ability", 3)
	}

	if firstT == nil {
		return
	}
	ctx.TargetFirstInvolvementT = float64Ptr(*firstT)
	ctx.TargetFirstInvolvementSource = firstSource
	delay := *firstT - startT
	ctx.SecondsToFirstInvolvement = float64Ptr(delay)

	for _, event := range tl.Deaths {
		if event.T < startT || event.T >= *firstT || event.VictimSlot == nil {
			continue
		}
		victim := tl.Players[slotKey(*event.VictimSlot)]
		if victim == nil || victim.Team != targetTeam || victim.PlayerSlot == targetSlot {
			continue
		}
		ctx.AlliedDeathsBeforeTargetInvolvement = append(ctx.AlliedDeathsBeforeTargetInvolvement, victim.PlayerSlot)
	}
	sort.Ints(ctx.AlliedDeathsBeforeTargetInvolvement)
}

func float64Ptr(v float64) *float64 {
	return &v
}

func intSliceContains(values []int, target int) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
