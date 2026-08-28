package timeline

import (
	"math"
	"sort"
)

const deathContextSampleMaxAge = 4.0

// TargetDeathContext is deterministic replay-derived context for one death of
// the configured player. It deliberately contains evidence only; whether the
// death was isolated, avoidable, or otherwise a mistake belongs to a later
// detector layer.
type TargetDeathContext struct {
	T                 float64 `json:"t"`
	PositionAvailable bool    `json:"position_available"`
	X                 float64 `json:"x,omitempty"`
	Y                 float64 `json:"y,omitempty"`

	KillerSlot  *int  `json:"killer_slot,omitempty"`
	AssistSlots []int `json:"assist_slots,omitempty"`

	Fight        *DeathFightContext `json:"fight,omitempty"`
	NearbyAllies []NearbyAlly       `json:"nearby_allies"`

	DamageReceivedLast5s  int64 `json:"damage_received_last_5s"`
	DamageReceivedLast10s int64 `json:"damage_received_last_10s"`
	DamageDealtLast5s     int64 `json:"damage_dealt_last_5s"`
	DamageDealtLast10s    int64 `json:"damage_dealt_last_10s"`

	EnemyKnowledge []EnemyKnowledgeState `json:"enemy_knowledge"`
}

// DeathFightContext copies the final consolidated fight window associated with
// the target death. TimeFromFightStart is measured from the padded final fight
// start because that is the stable window exposed by MatchTimeline.
type DeathFightContext struct {
	StartT               float64 `json:"start_t"`
	EndT                 float64 `json:"end_t"`
	CenterX               float64 `json:"center_x,omitempty"`
	CenterY               float64 `json:"center_y,omitempty"`
	Participants          []int   `json:"participants"`
	Deaths                int     `json:"deaths"`
	HeroDamage            int64   `json:"hero_damage"`
	TargetInvolved        bool    `json:"target_involved"`
	TimeFromFightStart    float64 `json:"time_from_fight_start"`
}

// NearbyAlly is one alive allied primary hero with a sufficiently fresh replay
// sample at the target death time. Samples are sorted by distance so callers
// can read immediate support context without inventing a classification.
type NearbyAlly struct {
	PlayerSlot int     `json:"player_slot"`
	SampleT    float64 `json:"sample_t"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Distance   float64 `json:"distance"`
}

// DeriveTargetDeathContexts builds one context per target-player death from the
// already-derived final fight, player-sample, damage, and conservative enemy
// knowledge layers. It never reads an enemy's actual position at the death
// time; enemy state comes only from EnemyKnowledgeAt.
func DeriveTargetDeathContexts(tl *MatchTimeline) []TargetDeathContext {
	if tl == nil {
		return []TargetDeathContext{}
	}

	target := tl.Players[slotKey(tl.TargetPlayerSlot)]
	if target == nil {
		return []TargetDeathContext{}
	}

	out := make([]TargetDeathContext, 0)
	for _, death := range tl.Deaths {
		if death.VictimSlot == nil || *death.VictimSlot != tl.TargetPlayerSlot {
			continue
		}

		ctx := TargetDeathContext{
			T:              death.T,
			AssistSlots:    append([]int(nil), death.AssistSlots...),
			NearbyAllies:   []NearbyAlly{},
			EnemyKnowledge: []EnemyKnowledgeState{},
		}
		if death.AttackerSlot != nil {
			killer := *death.AttackerSlot
			ctx.KillerSlot = &killer
		}
		sort.Ints(ctx.AssistSlots)

		if sample, ok := nearestHeroSampleAt(target, death.T); ok {
			ctx.PositionAvailable = true
			ctx.X = sample.X
			ctx.Y = sample.Y
			ctx.Fight = fightContextForDeath(tl.Fights, tl.TargetPlayerSlot, death.T, sample.X, sample.Y, true)
			ctx.NearbyAllies = nearbyAlliesAt(tl, target.Team, tl.TargetPlayerSlot, death.T, sample.X, sample.Y)
		} else {
			ctx.Fight = fightContextForDeath(tl.Fights, tl.TargetPlayerSlot, death.T, 0, 0, false)
		}

		ctx.DamageReceivedLast5s, ctx.DamageReceivedLast10s,
			ctx.DamageDealtLast5s, ctx.DamageDealtLast10s = damageContextAt(tl.Damage, tl.TargetPlayerSlot, death.T)
		ctx.EnemyKnowledge = enemyKnowledgeAt(tl, target.Team, death.T)
		out = append(out, ctx)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

func fightContextForDeath(fights []FightWindow, targetSlot int, t, x, y float64, hasPosition bool) *DeathFightContext {
	best := -1
	bestDistance := math.MaxFloat64

	for i := range fights {
		fight := &fights[i]
		if t < fight.StartT || t > fight.EndT || !fightContainsSlot(*fight, targetSlot) {
			continue
		}

		if !hasPosition {
			if best < 0 {
				best = i
			}
			continue
		}

		distance := math.Hypot(x-fight.CenterX, y-fight.CenterY)
		if best < 0 || distance < bestDistance {
			best = i
			bestDistance = distance
		}
	}

	if best < 0 {
		return nil
	}
	fight := fights[best]
	return &DeathFightContext{
		StartT:            fight.StartT,
		EndT:              fight.EndT,
		CenterX:            fight.CenterX,
		CenterY:            fight.CenterY,
		Participants:       append([]int(nil), fight.Participants...),
		Deaths:             fight.Deaths,
		HeroDamage:         fight.HeroDamage,
		TargetInvolved:     fight.TargetInvolved,
		TimeFromFightStart: t - fight.StartT,
	}
}

func fightContainsSlot(fight FightWindow, slot int) bool {
	if fight.TargetInvolved {
		for _, participant := range fight.Participants {
			if participant == slot {
				return true
			}
		}
		return false
	}
	for _, participant := range fight.Participants {
		if participant == slot {
			return true
		}
	}
	return false
}

func nearbyAlliesAt(tl *MatchTimeline, team, targetSlot int, t, targetX, targetY float64) []NearbyAlly {
	out := make([]NearbyAlly, 0, 4)
	for _, player := range tl.Players {
		if player == nil || player.PlayerSlot == targetSlot || player.Team != team {
			continue
		}
		sample, ok := nearestHeroSampleAt(player, t)
		if !ok || !sample.Alive {
			continue
		}
		out = append(out, NearbyAlly{
			PlayerSlot: player.PlayerSlot,
			SampleT:    sample.T,
			X:          sample.X,
			Y:          sample.Y,
			Distance:   math.Hypot(sample.X-targetX, sample.Y-targetY),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance == out[j].Distance {
			return out[i].PlayerSlot < out[j].PlayerSlot
		}
		return out[i].Distance < out[j].Distance
	})
	return out
}

func nearestHeroSampleAt(player *PlayerTimeline, t float64) (HeroSample, bool) {
	if player == nil || len(player.Samples) == 0 {
		return HeroSample{}, false
	}

	samples := player.Samples
	i := sort.Search(len(samples), func(i int) bool { return samples[i].T >= t })
	best := -1
	bestDelta := math.MaxFloat64
	if i < len(samples) {
		best = i
		bestDelta = math.Abs(samples[i].T - t)
	}
	if i > 0 {
		delta := math.Abs(samples[i-1].T - t)
		if delta <= bestDelta {
			best = i - 1
			bestDelta = delta
		}
	}
	if best < 0 || bestDelta > deathContextSampleMaxAge {
		return HeroSample{}, false
	}
	return samples[best], true
}

func damageContextAt(damage []DamageEvent, targetSlot int, t float64) (received5, received10, dealt5, dealt10 int64) {
	for _, event := range damage {
		if event.Value <= 0 || event.T > t || event.T < t-10.0 {
			continue
		}
		value := int64(event.Value)
		if event.VictimSlot == targetSlot {
			received10 += value
			if event.T >= t-5.0 {
				received5 += value
			}
		}
		if event.AttackerSlot == targetSlot {
			dealt10 += value
			if event.T >= t-5.0 {
				dealt5 += value
			}
		}
	}
	return received5, received10, dealt5, dealt10
}

func enemyKnowledgeAt(tl *MatchTimeline, targetTeam int, t float64) []EnemyKnowledgeState {
	slots := make([]int, 0, 5)
	for _, player := range tl.Players {
		if player == nil || player.Team == targetTeam {
			continue
		}
		slots = append(slots, player.PlayerSlot)
	}
	sort.Ints(slots)

	out := make([]EnemyKnowledgeState, 0, len(slots))
	for _, slot := range slots {
		out = append(out, EnemyKnowledgeAt(tl.Knowledge, slot, t))
	}
	return out
}
