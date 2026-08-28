package timeline

// LaneWaveTimeline is a deterministic, compact reconstruction of lane-wave
// identity from replay-observed creep activation cohorts and positions. It is
// derived context, not a coaching judgment. UnknownTrackCount is explicit so
// later detectors can fail closed instead of treating an unclassified track as
// evidence that no wave existed.
type LaneWaveTimeline struct {
	Available                    bool                       `json:"available"`
	Method                       string                     `json:"method"`
	SampleIntervalSeconds        float64                    `json:"sample_interval_seconds"`
	SpawnCohortRoundingSeconds   float64                    `json:"spawn_cohort_rounding_seconds"`
	ComponentRadiusWorld         float64                    `json:"component_radius_world"`
	TrackLinkRadiusWorld         float64                    `json:"track_link_radius_world"`
	LaneEvidenceDistanceWorld    float64                    `json:"lane_evidence_distance_world"`
	UnknownTrackCount            int                        `json:"unknown_track_count"`
	ActivationEvidence           LaneWaveActivationEvidence `json:"activation_evidence"`
	Waves                        []LaneWave                 `json:"waves"`
}

// LaneWaveActivationEvidence records how spawn cohorts were anchored. A
// waiting->active transition is the strongest replay evidence. CreatedActive
// and FirstObservedActive are retained separately so patch/demo differences are
// visible rather than silently treated as equivalent facts.
type LaneWaveActivationEvidence struct {
	WaitingTransitions int `json:"waiting_transitions"`
	CreatedActive      int `json:"created_active"`
	FirstObservedActive int `json:"first_observed_active"`
}

// LaneWave identifies one team's spawn cohort on one lane. SpawnT is the
// nearest-second observed activation cohort, not a claim about Dota's schedule.
// Multiple temporary spatial track fragments may be merged into one wave after
// lane classification; TrackFragments exposes that reconstruction detail.
type LaneWave struct {
	ID             string           `json:"id"`
	Team           int              `json:"team"`
	SpawnT         float64          `json:"spawn_t"`
	Lane           string           `json:"lane"` // top | mid | bottom
	StartT         float64          `json:"start_t"`
	EndT           float64          `json:"end_t"`
	TrackFragments int              `json:"track_fragments"`
	Samples        []LaneWaveSample `json:"samples"`
}

// LaneWaveSample is the compact spatial state of one reconstructed wave at a
// one-second boundary. Coordinates use the same Source 2 cell-coordinate scale
// as HeroSample and CreepCluster.
type LaneWaveSample struct {
	T                float64 `json:"t"`
	CenterX          float64 `json:"center_x"`
	CenterY          float64 `json:"center_y"`
	CreepCount       int     `json:"creep_count"`
	LaneCreepCount   int     `json:"lane_creep_count"`
	SiegeCreepCount  int     `json:"siege_creep_count"`
}
