package coaching

// KeyAbilityReviewEvidence is the compact provider-facing representation of one
// supported target key-ability use. Decision-time state and retrospective
// outcome facts remain explicitly separated by field naming.
type KeyAbilityReviewEvidence struct {
	Ability                        string   `json:"ability"`
	CastT                          float64  `json:"cast_t"`
	TargetSampleAvailable          bool     `json:"target_sample_available"`
	TargetAliveAtCast              bool     `json:"target_alive_at_cast"`
	TargetHPAtCast                 int32    `json:"target_hp_at_cast,omitempty"`
	TargetMaxHPAtCast              int32    `json:"target_max_hp_at_cast,omitempty"`
	TargetHPPctAtCast              *float64 `json:"target_hp_pct_at_cast,omitempty"`
	AlliedTeammatesAliveAtCast     int      `json:"allied_teammates_alive_at_cast"`
	PreCastWindowSeconds           float64  `json:"pre_cast_window_seconds"`
	TargetDamageDealtBeforeCast    int64    `json:"target_damage_dealt_before_cast"`
	TargetDamageReceivedBeforeCast int64    `json:"target_damage_received_before_cast"`
	OutcomeWindowSeconds           float64  `json:"outcome_window_seconds"`
	TargetDamageDealtAfterCast     int64    `json:"target_damage_dealt_after_cast"`
	TargetDamageReceivedAfterCast  int64    `json:"target_damage_received_after_cast"`
	EnemyDeathsAfterCast           int      `json:"enemy_deaths_after_cast"`
	AlliedDeathsAfterCast          int      `json:"allied_deaths_after_cast"`
	TargetDeathT                   *float64 `json:"target_death_t,omitempty"`
	TargetDeathInflictor           string   `json:"target_death_inflictor,omitempty"`
}

// ActiveDamageReflectReviewEvidence is a narrow compact interaction. The item
// activation is replay truth, not automatically player knowledge; that status
// is carried explicitly across the provider boundary.
type ActiveDamageReflectReviewEvidence struct {
	Ability                  string   `json:"ability"`
	CastT                    float64  `json:"cast_t"`
	Item                     string   `json:"item"`
	ItemUseT                 float64  `json:"item_use_t"`
	SecondsFromItemUseToCast float64  `json:"seconds_from_item_use_to_cast"`
	PlayerKnowledgeStatus    string   `json:"player_knowledge_status"`
	OutcomeWindowSeconds     float64  `json:"outcome_window_seconds"`
	ReflectedDamageAfterCast int64    `json:"reflected_damage_after_cast"`
	FirstReflectedDamageT    *float64 `json:"first_reflected_damage_t,omitempty"`
	TargetDeathToReflect     bool     `json:"target_death_to_reflect"`
	TargetDeathT             *float64 `json:"target_death_t,omitempty"`
}

// TimeWalkDamageRecoveryReviewEvidence keeps periodic hero samples explicitly
// separate from the exact cast timestamp. Pre-cast damage and the latest sample
// are decision-time evidence; the first later sample, later damage, and death
// are retrospective only.
type TimeWalkDamageRecoveryReviewEvidence struct {
	Ability                     string   `json:"ability"`
	CastT                       float64  `json:"cast_t"`
	TargetSampleAvailable       bool     `json:"target_sample_available"`
	TargetAliveAtCast           bool     `json:"target_alive_at_cast"`
	TargetSampleT               float64  `json:"target_sample_t"`
	TargetSampleAgeSeconds      float64  `json:"target_sample_age_seconds"`
	TargetHPAtCastSample        int32    `json:"target_hp_at_cast_sample"`
	TargetMaxHPAtCastSample     int32    `json:"target_max_hp_at_cast_sample"`
	TargetHPPctAtCastSample     *float64 `json:"target_hp_pct_at_cast_sample,omitempty"`
	PreDamageWindowSeconds      float64  `json:"pre_damage_window_seconds"`
	IncomingDamageBeforeCast    int64    `json:"incoming_damage_before_cast"`
	IncomingDamagePctMaxHP      float64  `json:"incoming_damage_pct_max_hp"`
	PostCastSampleWindowSeconds float64  `json:"post_cast_sample_window_seconds"`
	PostCastSampleAvailable     bool     `json:"post_cast_sample_available"`
	PostCastSampleT             *float64 `json:"post_cast_sample_t,omitempty"`
	PostCastSampleDelaySeconds  *float64 `json:"post_cast_sample_delay_seconds,omitempty"`
	TargetHPAtPostCastSample    int32    `json:"target_hp_at_post_cast_sample,omitempty"`
	TargetMaxHPAtPostCastSample int32    `json:"target_max_hp_at_post_cast_sample,omitempty"`
	TargetHPPctAtPostCastSample *float64 `json:"target_hp_pct_at_post_cast_sample,omitempty"`
	OutcomeWindowSeconds        float64  `json:"outcome_window_seconds"`
	IncomingDamageAfterCast     int64    `json:"incoming_damage_after_cast"`
	TargetDeathT                *float64 `json:"target_death_t,omitempty"`
}
