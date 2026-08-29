package timeline

import (
	"fmt"
	"math"
	"sort"
)

const (
	targetWaveDangerMethod              = "causal_wave_taking_structure_support_knowledge_v1"
	targetWaveDangerSampleMaxAgeSeconds = 1.5
)

// TargetWaveDangerTimeline is deterministic evidence for later dangerous-wave
// judgment. It deliberately emits no candidate and defines no danger threshold.
// In particular, lane progress is left unavailable until replay/map geometry
// provides a validated forward-position model.
type TargetWaveDangerTimeline struct {
	Available                 bool                      `json:"available"`
	Method                    string                    `json:"method"`
	SampleMaxAgeSeconds       float64                   `json:"sample_max_age_seconds"`
	LaneProgressAvailable     bool                      `json:"lane_progress_available"`
	LaneProgressUnavailableAs string                    `json:"lane_progress_unavailable_as,omitempty"`
	Contexts                  []TargetWaveDangerContext `json:"contexts"`
}

// TargetWaveDangerContext contains one evidence bundle for an accepted M14
// wave-taking period. The wave-taking fields are copied as compact audit anchors
// instead of embedding the full M14 period or M13 wave track.
type TargetWaveDangerContext struct {
	WaveID          string  `json:"wave_id"`
	Lane            string  `json:"lane"`
	EnemyTeam       int     `json:"enemy_team"`
	FriendlyTeam    int     `json:"friendly_team"`
	SpawnT          float64 `json:"spawn_t"`
	StartT          float64 `json:"start_t"`
	EndT            float64 `json:"end_t"`
	ExposureStartT  float64 `json:"exposure_start_t"`
	ExposureEndT    float64 `json:"exposure_end_t"`
	FirstDepletionT float64 `json:"first_depletion_t"`
	LastDepletionT  float64 `json:"last_depletion_t"`
	ObservedCreepLoss int   `json:"observed_creep_loss"`

	Snapshots []TargetWaveDangerSnapshot `json:"snapshots"`
}

// TargetWaveDangerSnapshot is a causal point-in-time view at a meaningful
// wave-taking boundary. EnemyKnowledge is obtained only through
// EnemyKnowledgeAt; actual enemy replay positions are never copied here.
type TargetWaveDangerSnapshot struct {
	Kind string  `json:"kind"` // start | first_depletion | last_depletion | end | exposure_end
	T    float64 `json:"t"`

	TargetAvailable  bool    `json:"target_available"`
	TargetSampleT    float64 `json:"target_sample_t,omitempty"`
	TargetSampleAge  float64 `json:"target_sample_age_seconds,omitempty"`
	TargetX          float64 `json:"target_x,omitempty"`
	TargetY          float64 `json:"target_y,omitempty"`
	TargetAlive      bool    `json:"target_alive,omitempty"`

	WaveAvailable bool    `json:"wave_available"`
	WaveSampleT   float64 `json:"wave_sample_t,omitempty"`
	WaveSampleAge float64 `json:"wave_sample_age_seconds,omitempty"`
	WaveX         float64 `json:"wave_x,omitempty"`
	WaveY         float64 `json:"wave_y,omitempty"`
	CreepCount    int     `json:"creep_count,omitempty"`

	NearbyAllies []WaveTakingNearbyAlly `json:"nearby_allies"`

	FriendlyStructuresAvailable bool               `json:"friendly_structures_available"`
	FriendlyStructures          LaneStructureState `json:"friendly_structures,omitempty"`
	EnemyStructuresAvailable    bool               `json:"enemy_structures_available"`
	EnemyStructures             LaneStructureState `json:"enemy_structures,omitempty"`

	EnemyKnowledge []EnemyKnowledgeState `json:"enemy_knowledge"`
}

// WaveTakingNearbyAlly records every alive allied primary hero with a fresh
// causal sample. No support-radius threshold is applied at this evidence stage.
type WaveTakingNearbyAlly struct {
	PlayerSlot       int     `json:"player_slot"`
	SampleT          float64 `json:"sample_t"`
	SampleAgeSeconds float64 `json:"sample_age_seconds"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	DistanceWorld    float64 `json:"distance_world"`
}

// DeriveTargetWaveDangerContext builds causal evidence only. Judgment gates are
// intentionally deferred until distributions and edge cases are validated on
// real replays.
func DeriveTargetWaveDangerContext(tl *MatchTimeline) TargetWaveDangerTimeline {
	out := TargetWaveDangerTimeline{
		Method:                    targetWaveDangerMethod,
		SampleMaxAgeSeconds:       targetWaveDangerSampleMaxAgeSeconds,
		LaneProgressAvailable:     false,
		LaneProgressUnavailableAs: "no validated causal lane-progress/map-geometry model",
		Contexts:                  []TargetWaveDangerContext{},
	}
	if tl == nil || !tl.TargetWaveTaking.Available {
		return out
	}

	target := tl.Players[fmt.Sprintf("%d", tl.TargetPlayerSlot)]
	if target == nil || (target.Team != 2 && target.Team != 3) {
		return out
	}
	out.Available = true

	waves := make(map[string]*LaneWave, len(tl.LaneWaves.Waves))
	for i := range tl.LaneWaves.Waves {
		wave := &tl.LaneWaves.Waves[i]
		waves[wave.ID] = wave
	}

	for _, period := range tl.TargetWaveTaking.Periods {
		ctx := TargetWaveDangerContext{
			WaveID:            period.WaveID,
			Lane:              period.Lane,
			EnemyTeam:         period.EnemyTeam,
			FriendlyTeam:      target.Team,
			SpawnT:            period.SpawnT,
			StartT:            period.StartT,
			EndT:              period.EndT,
			ExposureStartT:    period.ExposureStartT,
			ExposureEndT:      period.ExposureEndT,
			FirstDepletionT:   period.FirstDepletionT,
			LastDepletionT:    period.LastDepletionT,
			ObservedCreepLoss: period.ObservedCreepLoss,
			Snapshots:         []TargetWaveDangerSnapshot{},
		}

		anchors := []struct {
			kind string
			t    float64
		}{
			{kind: "start", t: period.StartT},
			{kind: "first_depletion", t: period.FirstDepletionT},
			{kind: "last_depletion", t: period.LastDepletionT},
			{kind: "end", t: period.EndT},
			{kind: "exposure_end", t: period.ExposureEndT},
		}
		seenT := make(map[float64]bool, len(anchors))
		for _, anchor := range anchors {
			if seenT[anchor.t] {
				continue
			}
			seenT[anchor.t] = true
			ctx.Snapshots = append(ctx.Snapshots, targetWaveDangerSnapshotAt(tl, target, waves[period.WaveID], period.Lane, period.EnemyTeam, anchor.kind, anchor.t))
		}
		out.Contexts = append(out.Contexts, ctx)
	}

	sort.SliceStable(out.Contexts, func(i, j int) bool {
		if out.Contexts[i].StartT != out.Contexts[j].StartT {
			return out.Contexts[i].StartT < out.Contexts[j].StartT
		}
		return out.Contexts[i].WaveID < out.Contexts[j].WaveID
	})
	return out
}

func targetWaveDangerSnapshotAt(tl *MatchTimeline, target *PlayerTimeline, wave *LaneWave, lane string, enemyTeam int, kind string, t float64) TargetWaveDangerSnapshot {
	s := TargetWaveDangerSnapshot{
		Kind:           kind,
		T:              t,
		NearbyAllies:   []WaveTakingNearbyAlly{},
		EnemyKnowledge: enemyKnowledgeAt(tl, target.Team, t),
	}

	if sample, ok := freshHeroSampleAtOrBefore(target, t, targetWaveDangerSampleMaxAgeSeconds); ok {
		s.TargetAvailable = true
		s.TargetSampleT = sample.T
		s.TargetSampleAge = t - sample.T
		s.TargetX = sample.X
		s.TargetY = sample.Y
		s.TargetAlive = sample.Alive
		if sample.Alive {
			s.NearbyAllies = waveTakingNearbyAlliesAt(tl, target.Team, target.PlayerSlot, t, sample.X, sample.Y)
		}
	}

	if sample, ok := laneWaveSampleAtOrBefore(wave, t, targetWaveDangerSampleMaxAgeSeconds); ok {
		s.WaveAvailable = true
		s.WaveSampleT = sample.T
		s.WaveSampleAge = t - sample.T
		s.WaveX = sample.CenterX
		s.WaveY = sample.CenterY
		s.CreepCount = sample.CreepCount
	}

	if state, ok := LaneStructureStateAt(tl.LaneStructures, target.Team, lane, t); ok {
		s.FriendlyStructuresAvailable = true
		s.FriendlyStructures = state
	}
	if state, ok := LaneStructureStateAt(tl.LaneStructures, enemyTeam, lane, t); ok {
		s.EnemyStructuresAvailable = true
		s.EnemyStructures = state
	}
	return s
}

func laneWaveSampleAtOrBefore(wave *LaneWave, t, maxAge float64) (LaneWaveSample, bool) {
	if wave == nil || len(wave.Samples) == 0 {
		return LaneWaveSample{}, false
	}
	i := sort.Search(len(wave.Samples), func(i int) bool { return wave.Samples[i].T > t })
	if i == 0 {
		return LaneWaveSample{}, false
	}
	sample := wave.Samples[i-1]
	if t-sample.T > maxAge {
		return LaneWaveSample{}, false
	}
	return sample, true
}

func waveTakingNearbyAlliesAt(tl *MatchTimeline, team, targetSlot int, t, targetX, targetY float64) []WaveTakingNearbyAlly {
	out := make([]WaveTakingNearbyAlly, 0, 4)
	for _, player := range tl.Players {
		if player == nil || player.PlayerSlot == targetSlot || player.Team != team {
			continue
		}
		sample, ok := freshHeroSampleAtOrBefore(player, t, targetWaveDangerSampleMaxAgeSeconds)
		if !ok || !sample.Alive {
			continue
		}
		out = append(out, WaveTakingNearbyAlly{
			PlayerSlot:       player.PlayerSlot,
			SampleT:          sample.T,
			SampleAgeSeconds: t - sample.T,
			X:                sample.X,
			Y:                sample.Y,
			DistanceWorld:    math.Hypot(sample.X-targetX, sample.Y-targetY) * worldUnitsPerTimelineCoord,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DistanceWorld != out[j].DistanceWorld {
			return out[i].DistanceWorld < out[j].DistanceWorld
		}
		return out[i].PlayerSlot < out[j].PlayerSlot
	})
	return out
}
