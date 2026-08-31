package coaching

// ChronosphereKeyAbilityReviewEvidence keeps the existing generic key-ability
// evidence intact while adding one compact, hero-specific context block. The
// embedded generic fields preserve the provider contract used by other key
// abilities such as Time Walk.
type ChronosphereKeyAbilityReviewEvidence struct {
	KeyAbilityReviewEvidence
	ChronosphereFollowup ChronosphereFollowupReviewEvidence `json:"chronosphere_followup"`
}

// ChronosphereFollowupReviewEvidence contains only aggregate damage
// relationships. It deliberately exposes no player slots, hero identities, or
// coordinates, and carries explicit negative capability flags so downstream
// review cannot silently treat follow-up damage as proof of sphere placement or
// caught units.
type ChronosphereFollowupReviewEvidence struct {
	FollowupWindowSeconds                           float64  `json:"followup_window_seconds"`
	FollowupWindowEqualsSpellDuration               bool     `json:"followup_window_equals_spell_duration"`
	CaughtHeroesConfirmedFromReplay                 bool     `json:"caught_heroes_confirmed_from_replay"`
	CastPlacementConfirmedFromReplay                bool     `json:"cast_placement_confirmed_from_replay"`
	RecentEnemyInteractorsBeforeCast                int      `json:"recent_enemy_interactors_before_cast"`
	RecentAlliedTeammatesInteractingWithSameEnemies int      `json:"recent_allied_teammates_interacting_with_same_enemies_before_cast"`
	TargetEnemyHeroesDamagedInFollowup              int      `json:"target_enemy_heroes_damaged_in_followup"`
	TargetHeroDamageInFollowup                      int64    `json:"target_hero_damage_in_followup"`
	AlliedTeammatesDamagingTargetVictimsInFollowup  int      `json:"allied_teammates_damaging_target_victims_in_followup"`
	AlliedHeroDamageToTargetVictimsInFollowup       int64    `json:"allied_hero_damage_to_target_victims_in_followup"`
	SecondsToFirstTargetHeroDamageAfterCast          *float64 `json:"seconds_to_first_target_hero_damage_after_cast,omitempty"`
}
