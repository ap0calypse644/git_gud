package timeline

// MatchTimeline is the deterministic replay-derived input used by later
// decision detectors. It intentionally contains no coaching judgments.
type MatchTimeline struct {
	MatchID          int64                       `json:"match_id"`
	AccountID        uint32                      `json:"account_id"`
	TargetPlayerSlot int                         `json:"target_player_slot"`
	GameBuild        uint32                      `json:"game_build"`
	DurationSeconds  float64                     `json:"duration_seconds"`
	Players          map[string]*PlayerTimeline `json:"players"`
	Deaths           []DeathEvent                `json:"deaths,omitempty"`
	Damage           []DamageEvent               `json:"damage,omitempty"`
	Abilities        []AbilityEvent              `json:"abilities,omitempty"`
	Items            []ItemEvent                 `json:"items,omitempty"`
	Buybacks         []BuybackEvent              `json:"buybacks,omitempty"`
	Objectives       []ObjectiveEvent            `json:"objectives,omitempty"`
	Fights           []FightWindow               `json:"fights,omitempty"`
}

type PlayerTimeline struct {
	PlayerSlot int          `json:"player_slot"`
	PlayerID   int          `json:"player_id"`
	Team       int          `json:"team"`
	HeroClass  string       `json:"hero_class,omitempty"`
	HeroName   string       `json:"hero_name,omitempty"`
	Samples    []HeroSample `json:"samples"`
}

// HeroSample is a roughly 1 Hz snapshot from the replay. X/Y use Source 2's
// cell-coordinate scale (roughly the 64..192 range over the playable Dota
// map), which is convenient for later lane/wave/minimap reasoning.
type HeroSample struct {
	T       float64 `json:"t"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	HP      int32   `json:"hp"`
	MaxHP   int32   `json:"max_hp"`
	Mana    float32 `json:"mana"`
	MaxMana float32 `json:"max_mana"`
	Level   int32   `json:"level"`
	Alive   bool    `json:"alive"`
}

type DeathEvent struct {
	T            float64 `json:"t"`
	Attacker     string  `json:"attacker,omitempty"`
	Victim       string  `json:"victim,omitempty"`
	Inflictor    string  `json:"inflictor,omitempty"`
	AttackerSlot *int    `json:"attacker_slot,omitempty"`
	VictimSlot   *int    `json:"victim_slot,omitempty"`
	AssistSlots  []int   `json:"assist_slots,omitempty"`
}

// DamageEvent is one hero-to-hero combat-log damage record. DamageType is the
// raw Dota combat-log bitfield; keeping it raw avoids baking patch-specific
// interpretation into the parser layer.
type DamageEvent struct {
	T            float64 `json:"t"`
	Attacker     string  `json:"attacker"`
	Victim       string  `json:"victim"`
	Inflictor    string  `json:"inflictor,omitempty"`
	AttackerSlot int     `json:"attacker_slot"`
	VictimSlot   int     `json:"victim_slot"`
	Value        int32   `json:"value"`
	DamageType   uint32  `json:"damage_type,omitempty"`
}

type AbilityEvent struct {
	T          float64 `json:"t"`
	PlayerSlot int     `json:"player_slot"`
	Hero       string  `json:"hero"`
	Ability    string  `json:"ability"`
}

// ItemEvent distinguishes purchases from item-use combat-log events.
type ItemEvent struct {
	T          float64 `json:"t"`
	PlayerSlot int     `json:"player_slot"`
	Hero       string  `json:"hero"`
	Item       string  `json:"item"`
	Action     string  `json:"action"` // purchase | use
}

type BuybackEvent struct {
	T          float64 `json:"t"`
	PlayerSlot int     `json:"player_slot"`
}

type ObjectiveEvent struct {
	T            float64 `json:"t"`
	Type         string  `json:"type"`
	Actor        string  `json:"actor,omitempty"`
	Target       string  `json:"target,omitempty"`
	AttackerTeam int     `json:"attacker_team,omitempty"`
	TargetTeam   int     `json:"target_team,omitempty"`
}

// FightWindow is derived deterministically from hero-to-hero damage, deaths,
// and replay positions. It is context, not a coaching judgment. CenterX/Y are
// the approximate center of the observed combat in Source 2 cell coordinates.
type FightWindow struct {
	StartT         float64 `json:"start_t"`
	EndT           float64 `json:"end_t"`
	CenterX        float64 `json:"center_x,omitempty"`
	CenterY        float64 `json:"center_y,omitempty"`
	Participants   []int   `json:"participants"`
	Deaths         int     `json:"deaths"`
	HeroDamage     int64   `json:"hero_damage"`
	TargetInvolved bool    `json:"target_involved"`
}
