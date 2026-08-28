package timeline

// MatchTimeline is the deterministic replay-derived input used by later
// decision detectors. It intentionally contains no coaching judgments.
type MatchTimeline struct {
	MatchID             int64                       `json:"match_id"`
	AccountID           uint32                      `json:"account_id"`
	TargetPlayerSlot    int                         `json:"target_player_slot"`
	GameBuild           uint32                      `json:"game_build"`
	DurationSeconds     float64                     `json:"duration_seconds"`
	Players             map[string]*PlayerTimeline `json:"players"`
	Deaths              []DeathEvent                `json:"deaths,omitempty"`
	Damage              []DamageEvent               `json:"damage,omitempty"`
	Abilities           []AbilityEvent              `json:"abilities,omitempty"`
	Items               []ItemEvent                 `json:"items,omitempty"`
	Buybacks            []BuybackEvent              `json:"buybacks,omitempty"`
	Objectives          []ObjectiveEvent             `json:"objectives,omitempty"`
	Fights              []FightWindow                `json:"fights,omitempty"`
	CreepClusters       CreepClusterTimeline         `json:"creep_clusters"`
	LaneWaves           LaneWaveTimeline             `json:"lane_waves"`
	Visibility          VisibilityTimeline           `json:"visibility"`
	VisionSources       VisionSourceTimeline         `json:"vision_sources"`
	Knowledge           KnowledgeTimeline            `json:"knowledge"`
	TargetDeathContexts []TargetDeathContext         `json:"target_death_contexts"`
	TargetFightContexts []TargetFightContext         `json:"target_fight_contexts"`
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

// VisibilityTimeline stores only replay facts. DirectTeamMaskAvailable says
// whether this replay build actually transmitted m_iTaggedAsVisibleByTeam on
// hero entities. Some modern replay builds omit that field even though it
// exists in the live game schema, so absence is explicit rather than silently
// interpreted as "not visible".
type VisibilityTimeline struct {
	DirectTeamMaskAvailable bool              `json:"direct_team_mask_available"`
	Events                  []VisibilityEvent `json:"events"`
}

// VisibilityEvent is emitted when a hero's replay-provided team-visibility
// mask changes. Dota team IDs are also the mask bit numbers: bit 2 is Radiant
// and bit 3 is Dire. The booleans are derived from the raw mask for convenient
// validation; the raw value is retained so later patches can be audited.
type VisibilityEvent struct {
	T                 float64 `json:"t"`
	PlayerSlot        int     `json:"player_slot"`
	Team              int     `json:"team"`
	X                 float64 `json:"x,omitempty"`
	Y                 float64 `json:"y,omitempty"`
	VisibleByTeamMask int     `json:"visible_by_team_mask"`
	VisibleToRadiant  bool    `json:"visible_to_radiant"`
	VisibleToDire     bool    `json:"visible_to_dire"`
}

// VisionSourceTimeline contains replay-observed vision sources. These are raw
// inputs for later visibility estimates, not proof that an enemy was seen.
type VisionSourceTimeline struct {
	Wards  []WardInterval     `json:"wards"`
	Heroes []HeroVisionSample `json:"heroes"`
}

// WardInterval is one observer/sentry ward's active lifetime reconstructed
// from its replay entity. Vision ranges are the raw Source 2 world-unit values
// transmitted by the entity; X/Y use the timeline's cell-coordinate scale.
// EndReason is intentionally conservative: a life-state transition does not by
// itself tell us whether the ward was killed or naturally expired.
type WardInterval struct {
	EntityIndex      int32   `json:"entity_index"`
	EntitySerial     int32   `json:"entity_serial"`
	Kind             string  `json:"kind"` // observer | sentry
	Team             int     `json:"team"`
	OwnerRawPlayerID *int    `json:"owner_raw_player_id,omitempty"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	StartT           float64 `json:"start_t"`
	EndT             float64 `json:"end_t"`
	EndReason        string  `json:"end_reason"`
	DayVisionRange   float64 `json:"day_vision_range,omitempty"`
	NightVisionRange float64 `json:"night_vision_range,omitempty"`
	FOWTeam          int     `json:"fow_team,omitempty"`
}

// HeroVisionSample is a roughly 1 Hz replay fact for a primary hero acting as
// a potential team vision source. Illusions and clones are deliberately
// excluded for this first conservative source model.
type HeroVisionSample struct {
	T                float64 `json:"t"`
	PlayerSlot       int     `json:"player_slot"`
	Team             int     `json:"team"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	Alive            bool    `json:"alive"`
	DayVisionRange   float64 `json:"day_vision_range,omitempty"`
	NightVisionRange float64 `json:"night_vision_range,omitempty"`
}

// KnowledgeTimeline is deliberately weaker than direct visibility. Its
// intervals are geometry-derived estimates from replay-observed vision sources
// and must not be promoted into confirmed player knowledge.
type KnowledgeTimeline struct {
	Team                int                           `json:"team"`
	Method              string                        `json:"method"`
	EstimatedVisibility []EstimatedVisibilityInterval `json:"estimated_visibility"`
}

// EstimatedVisibilityInterval is a contiguous run of enemy replay samples that
// fall inside at least one friendly source's conservative nominal vision
// radius. Terrain, trees, invisibility, temporary darkness and other FoW
// mechanics can invalidate the estimate.
type EstimatedVisibilityInterval struct {
	PlayerSlot      int               `json:"player_slot"`
	StartT          float64           `json:"start_t"`
	EndT            float64           `json:"end_t"`
	StartX          float64           `json:"start_x"`
	StartY          float64           `json:"start_y"`
	EndX            float64           `json:"end_x"`
	EndY            float64           `json:"end_y"`
	SampleCount     int               `json:"sample_count"`
	SourceWards     []VisionSourceRef `json:"source_wards,omitempty"`
	SourceHeroSlots []int             `json:"source_hero_slots,omitempty"`
}

type VisionSourceRef struct {
	EntityIndex  int32 `json:"entity_index"`
	EntitySerial int32 `json:"entity_serial"`
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
// and replay positions. It is context, not a coaching judgment. StartT/EndT
// include the detector's lead/trail padding. ObservedStartT/ObservedEndT retain
// the first/last raw combat moment so later decision analysis can reason about
// ordering without reverse-engineering padding.
type FightWindow struct {
	StartT           float64 `json:"start_t"`
	EndT             float64 `json:"end_t"`
	ObservedStartT   float64 `json:"observed_start_t"`
	ObservedEndT     float64 `json:"observed_end_t"`
	CenterX          float64 `json:"center_x,omitempty"`
	CenterY          float64 `json:"center_y,omitempty"`
	Participants     []int   `json:"participants"`
	Deaths           int     `json:"deaths"`
	HeroDamage       int64   `json:"hero_damage"`
	TargetInvolved   bool    `json:"target_involved"`
}
