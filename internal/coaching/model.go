package coaching

// MatchCoachingInput is the only match-level model intended for eventual AI
// consumption. It deliberately excludes the raw replay timeline and contains
// only detector-normalized coaching moments.
type MatchCoachingInput struct {
	MatchID int64            `json:"match_id"`
	Hero    string           `json:"hero"`
	Moments []CoachingMoment `json:"moments"`
}

// CoachingMoment is one deterministic detector candidate prepared for a later
// explanation layer. Point detectors use equal StartT/EndT; interval detectors
// may expose a wider review span without forwarding the raw replay timeline.
type CoachingMoment struct {
	Type       string  `json:"type"`
	StartT     float64 `json:"start_t"`
	EndT       float64 `json:"end_t"`
	Confidence string  `json:"confidence"`
	Evidence   any     `json:"evidence"`
}

// PostWaveOverstayReviewEvidence is the compact, decision-safe evidence sent
// across the coaching boundary for an M17 post-wave overstay candidate. It
// excludes raw coordinates, absolute replay depth, exact enemy positions, and
// the wider M16/M17 context while retaining only explanation-relevant facts.
type PostWaveOverstayReviewEvidence struct {
	WaveID                           string   `json:"wave_id"`
	Lane                             string   `json:"lane"`
	LastDepletionT                   float64  `json:"last_depletion_t"`
	ExposureEndT                     float64  `json:"exposure_end_t"`
	PostClearDurationSeconds         float64  `json:"post_clear_duration_seconds"`
	PostClearLaneProgressDeltaWorld  float64  `json:"post_clear_lane_progress_delta_world"`
	TargetCombatStartedDuringPostClear bool   `json:"target_combat_started_during_post_clear"`
	SecondsFromClearToFirstInvolvement float64 `json:"seconds_from_clear_to_first_involvement"`
	TargetFirstInvolvementSource     string   `json:"target_first_involvement_source,omitempty"`
	NextWaveTakingObserved           bool     `json:"next_wave_taking_observed"`
	NextTargetDeathT                 *float64 `json:"next_target_death_t,omitempty"`
}

// ObjectiveReviewTowerOption is one causally exposed tower option that passed
// the detector's conservative actionability checks. No coordinates or creep
// geometry cross the coaching boundary.
type ObjectiveReviewTowerOption struct {
	Lane string `json:"lane"`
	Tier int    `json:"tier"`
}

// PostFightConversionReviewEvidence is the compact, decision-safe evidence sent
// across the coaching boundary for a post-fight conversion review candidate.
// It intentionally excludes raw hero positions, creep clusters, enemy victim
// identities, non-actionable front towers, and replay-only Roshan world state.
type PostFightConversionReviewEvidence struct {
	FightIndex                    int                          `json:"fight_index"`
	FightObservedStartT           float64                      `json:"fight_observed_start_t"`
	FightObservedEndT             float64                      `json:"fight_observed_end_t"`
	WindowStartT                  float64                      `json:"window_start_t"`
	WindowEndT                    float64                      `json:"window_end_t"`
	WindowEndReason               string                       `json:"window_end_reason"`
	WindowDurationSeconds         float64                      `json:"window_duration_seconds"`
	TargetAliveAtFightEnd         bool                         `json:"target_alive_at_fight_end"`
	AlliedHeroesAliveAtFightEnd   int                          `json:"allied_heroes_alive_at_fight_end"`
	AlliedDeaths                  int                          `json:"allied_deaths"`
	EnemyDeaths                   int                          `json:"enemy_deaths"`
	EnemyDeathAdvantage           int                          `json:"enemy_death_advantage"`
	EnemyDeathsStillDeadAtEnd     int                          `json:"enemy_deaths_still_dead_at_window_end"`
	PushableTowerOptions          []ObjectiveReviewTowerOption `json:"pushable_tower_options"`
	RoshanKnowledgeState          string                       `json:"roshan_knowledge_state,omitempty"`
	RoshanKnownAliveForDecision   bool                         `json:"roshan_known_alive_for_decision"`
	TargetTeamConversionCount     int                          `json:"target_team_conversion_count"`
	NoTargetTeamConversion        bool                         `json:"no_target_team_conversion"`
}
