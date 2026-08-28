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
// explanation layer. StartT and EndT are equal for the current point-in-time
// candidate detectors; interval detectors may use a wider span later.
type CoachingMoment struct {
	Type       string `json:"type"`
	StartT     float64 `json:"start_t"`
	EndT       float64 `json:"end_t"`
	Confidence string `json:"confidence"`
	Evidence   any    `json:"evidence"`
}
