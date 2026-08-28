package timeline

// MatchTimeline is the deterministic replay-derived input used by later
// decision detectors. It intentionally contains no coaching judgments.
type MatchTimeline struct {
	MatchID          int64                     `json:"match_id"`
	AccountID        uint32                    `json:"account_id"`
	TargetPlayerSlot int                       `json:"target_player_slot"`
	GameBuild        uint32                    `json:"game_build"`
	DurationSeconds  float64                   `json:"duration_seconds"`
	Players          map[string]*PlayerTimeline `json:"players"`
	Deaths           []DeathEvent              `json:"deaths,omitempty"`
}

type PlayerTimeline struct {
	PlayerSlot int          `json:"player_slot"`
	PlayerID   int          `json:"player_id"`
	Team       int          `json:"team"`
	HeroClass  string       `json:"hero_class,omitempty"`
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
	T         float64 `json:"t"`
	Attacker  string  `json:"attacker,omitempty"`
	Victim    string  `json:"victim,omitempty"`
	Inflictor string  `json:"inflictor,omitempty"`
}
