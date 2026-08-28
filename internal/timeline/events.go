package timeline

import (
	"strings"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"
)

type rawDamageEvent struct {
	timestamp  float64
	attacker   string
	victim     string
	inflictor  string
	value      int32
	damageType uint32
}

type rawNamedEvent struct {
	timestamp float64
	hero      string
	name      string
}

type rawBuybackEvent struct {
	timestamp        float64
	resourcePlayerID int
}

type rawObjectiveEvent struct {
	timestamp    float64
	typeName     string
	actor        string
	target       string
	attackerTeam int
	targetTeam   int
}

type eventCollector struct {
	parser             *manta.Parser
	heroNameIdx        *heroNameIndexReader
	heroNameToSlot     map[string]int
	slotToHeroName     map[int]string
	resourcePlayerTeam map[int]int
	damage             []rawDamageEvent
	abilities          []rawNamedEvent
	itemsUsed          []rawNamedEvent
	purchases          []rawNamedEvent
	buybacks           []rawBuybackEvent
	objectives         []rawObjectiveEvent
	wards              *wardCollector
}

func newEventCollector(p *manta.Parser) *eventCollector {
	c := &eventCollector{
		parser:             p,
		heroNameIdx:        newHeroNameIndexReader(),
		heroNameToSlot:     make(map[string]int),
		slotToHeroName:     make(map[int]string),
		resourcePlayerTeam: make(map[int]int),
		wards:              newWardCollector(),
	}

	// Ward entities are not heroes, so they never reach Parse's hero-specific
	// entity handler. Register a dedicated replay-fact listener here; event
	// collectors are installed before parsing begins and already own the other
	// non-hero replay facts.
	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		c.wards.observe(e, op, float64(p.NetTick)/tickRate)
		return nil
	})

	combatLogName := func(index uint32) string {
		name, _ := p.LookupStringByIndex("CombatLogNames", int32(index))
		return strings.TrimPrefix(name, "item_")
	}

	p.Callbacks.OnCMsgDOTACombatLogEntry(func(m *dota.CMsgDOTACombatLogEntry) error {
		timestamp := float64(m.GetTimestamp())
		switch m.GetType() {
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DAMAGE:
			c.damage = append(c.damage, rawDamageEvent{
				timestamp:  timestamp,
				attacker:   combatLogName(m.GetAttackerName()),
				victim:     combatLogName(m.GetTargetName()),
				inflictor:  combatLogName(m.GetInflictorName()),
				value:      int32(m.GetValue()),
				damageType: uint32(m.GetDamageType()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ABILITY:
			c.abilities = append(c.abilities, rawNamedEvent{
				timestamp: timestamp,
				hero:      combatLogName(m.GetAttackerName()),
				name:      combatLogName(m.GetInflictorName()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_ITEM:
			c.itemsUsed = append(c.itemsUsed, rawNamedEvent{
				timestamp: timestamp,
				hero:      combatLogName(m.GetAttackerName()),
				name:      combatLogName(m.GetInflictorName()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_PURCHASE:
			c.purchases = append(c.purchases, rawNamedEvent{
				timestamp: timestamp,
				hero:      combatLogName(m.GetTargetName()),
				name:      combatLogName(m.GetValue()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_BUYBACK:
			c.buybacks = append(c.buybacks, rawBuybackEvent{
				timestamp:        timestamp,
				resourcePlayerID: int(m.GetValue()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_FIRST_BLOOD:
			c.objectives = append(c.objectives, rawObjectiveEvent{
				timestamp:    timestamp,
				typeName:     "first_blood",
				attackerTeam: int(m.GetAttackerTeam()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_TEAM_BUILDING_KILL:
			c.objectives = append(c.objectives, rawObjectiveEvent{
				timestamp:    timestamp,
				typeName:     "building_kill",
				actor:        combatLogName(m.GetAttackerName()),
				target:       combatLogName(m.GetTargetName()),
				attackerTeam: int(m.GetAttackerTeam()),
				targetTeam:   int(m.GetTargetTeam()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_AEGIS_TAKEN:
			c.objectives = append(c.objectives, rawObjectiveEvent{
				timestamp: timestamp,
				typeName:  "aegis_taken",
				actor:     combatLogName(m.GetAttackerName()),
			})
		case dota.DOTA_COMBATLOG_TYPES_DOTA_COMBATLOG_DEATH:
			target := combatLogName(m.GetTargetName())
			switch {
			case strings.Contains(target, "npc_dota_roshan"):
				c.objectives = append(c.objectives, rawObjectiveEvent{
					timestamp: timestamp,
					typeName:  "roshan_kill",
					actor:     combatLogName(m.GetAttackerName()),
					target:    target,
				})
			case strings.Contains(target, "npc_dota_miniboss"):
				c.objectives = append(c.objectives, rawObjectiveEvent{
					timestamp: timestamp,
					typeName:  "tormentor_kill",
					actor:     combatLogName(m.GetAttackerName()),
					target:    target,
				})
			}
		}
		return nil
	})
	return c
}

func (c *eventCollector) observePlayerResource(e *manta.Entity) {
	for playerID := 0; playerID < 10; playerID++ {
		if team, ok := playerTeam(e, playerID); ok {
			c.resourcePlayerTeam[playerID] = team
		}
	}
}

func (c *eventCollector) observeHero(e *manta.Entity, slot int) {
	idx, ok := c.heroNameIdx.read(e.Get, e.Map)
	if !ok {
		return
	}
	name, ok := c.parser.LookupStringByIndex("EntityNames", idx)
	if !ok || name == "" {
		return
	}
	c.heroNameToSlot[name] = slot
	c.slotToHeroName[slot] = name
}

func (c *eventCollector) heroSlot(name string) (int, bool) {
	slot, ok := c.heroNameToSlot[name]
	return slot, ok
}

func (c *eventCollector) resourceSlot(playerID int) (int, bool) {
	team, ok := c.resourcePlayerTeam[playerID]
	if !ok {
		return 0, false
	}
	return resourcePlayerSlot(playerID, team)
}

func (c *eventCollector) apply(out *MatchTimeline, gameStartTime float64) {
	for slot, name := range c.slotToHeroName {
		if player := out.Players[slotKey(slot)]; player != nil {
			player.HeroName = name
		}
	}

	for _, raw := range c.damage {
		attackerSlot, attackerOK := c.heroSlot(raw.attacker)
		victimSlot, victimOK := c.heroSlot(raw.victim)
		if !attackerOK || !victimOK || attackerSlot == victimSlot || raw.value <= 0 {
			continue
		}
		out.Damage = append(out.Damage, DamageEvent{
			T:            raw.timestamp - gameStartTime,
			Attacker:     raw.attacker,
			Victim:       raw.victim,
			Inflictor:    raw.inflictor,
			AttackerSlot: attackerSlot,
			VictimSlot:   victimSlot,
			Value:        raw.value,
			DamageType:   raw.damageType,
		})
	}

	for _, raw := range c.abilities {
		slot, ok := c.heroSlot(raw.hero)
		if !ok || raw.name == "" {
			continue
		}
		out.Abilities = append(out.Abilities, AbilityEvent{
			T: raw.timestamp - gameStartTime, PlayerSlot: slot, Hero: raw.hero, Ability: raw.name,
		})
	}

	for _, raw := range c.itemsUsed {
		slot, ok := c.heroSlot(raw.hero)
		if !ok || raw.name == "" {
			continue
		}
		out.Items = append(out.Items, ItemEvent{
			T: raw.timestamp - gameStartTime, PlayerSlot: slot, Hero: raw.hero, Item: raw.name, Action: "use",
		})
	}
	for _, raw := range c.purchases {
		slot, ok := c.heroSlot(raw.hero)
		if !ok || raw.name == "" {
			continue
		}
		out.Items = append(out.Items, ItemEvent{
			T: raw.timestamp - gameStartTime, PlayerSlot: slot, Hero: raw.hero, Item: raw.name, Action: "purchase",
		})
	}

	for _, raw := range c.buybacks {
		slot, ok := c.resourceSlot(raw.resourcePlayerID)
		if !ok {
			continue
		}
		out.Buybacks = append(out.Buybacks, BuybackEvent{T: raw.timestamp - gameStartTime, PlayerSlot: slot})
	}

	for _, raw := range c.objectives {
		out.Objectives = append(out.Objectives, ObjectiveEvent{
			T:            raw.timestamp - gameStartTime,
			Type:         raw.typeName,
			Actor:        cleanObjectiveActor(raw.actor),
			Target:       raw.target,
			AttackerTeam: raw.attackerTeam,
			TargetTeam:   raw.targetTeam,
		})
	}

	c.wards.apply(out, gameStartTime, out.DurationSeconds)
}

func cleanObjectiveActor(actor string) string {
	if actor == "dota_unknown" {
		return ""
	}
	return actor
}

func slotKey(slot int) string {
	if slot < 0 {
		return ""
	}
	return strings.TrimSpace(intString(slot))
}

func intString(v int) string {
	// Small helper kept here to avoid fmt in this hot-path helper file.
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
