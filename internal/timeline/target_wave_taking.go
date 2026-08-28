package timeline

import (
	"fmt"
	"math"
	"sort"
)

const (
	targetWaveTakingMethod                   = "enemy_lane_wave_proximity_and_observed_depletion_v1"
	targetWaveTakingProximityRadiusWorld     = 1600.0
	targetWaveTakingProximityRadiusTimeline  = targetWaveTakingProximityRadiusWorld / 128.0
	targetWaveTakingSampleMaxAgeSeconds      = 1.5
	targetWaveTakingMaxContactGapSeconds     = 1.5
	targetWaveTakingMinContactSamples        = 3
	targetWaveTakingMinObservedCreepLoss     = 1
)

// TargetWaveTakingTimeline is deterministic derived context for periods where
// the configured player stayed close to an enemy lane wave while that wave was
// observed losing creeps. It is intentionally a conservative *candidate* for
// taking a wave, not proof that the player personally last-hit or damaged any
// creep. Later danger/overstay detectors may consume these periods as context.
type TargetWaveTakingTimeline struct {
	Available                 bool                     `json:"available"`
	Method                    string                   `json:"method"`
	ProximityRadiusWorld      float64                  `json:"proximity_radius_world"`
	SampleMaxAgeSeconds       float64                  `json:"sample_max_age_seconds"`
	MinContactSamples         int                      `json:"min_contact_samples"`
	MinObservedCreepLoss      int                      `json:"min_observed_creep_loss"`
	ExposurePeriodsObserved   int                      `json:"exposure_periods_observed"`
	RejectedTooShort          int                      `json:"rejected_too_short"`
	RejectedNoDepletion       int                      `json:"rejected_no_depletion"`
	Periods                   []TargetWaveTakingPeriod `json:"periods"`
}

// TargetWaveTakingPeriod summarizes one contiguous proximity run against one
// reconstructed enemy lane wave. ObservedCreepLoss is the sum of positive
// second-to-second creep-count decreases during the run; it does not attribute
// those deaths to the target player.
type TargetWaveTakingPeriod struct {
	WaveID                    string  `json:"wave_id"`
	Lane                      string  `json:"lane"`
	EnemyTeam                 int     `json:"enemy_team"`
	SpawnT                    float64 `json:"spawn_t"`
	StartT                    float64 `json:"start_t"`
	EndT                      float64 `json:"end_t"`
	DurationSeconds           float64 `json:"duration_seconds"`
	ContactSamples            int     `json:"contact_samples"`

	StartCreepCount           int `json:"start_creep_count"`
	EndCreepCount             int `json:"end_creep_count"`
	MinCreepCount             int `json:"min_creep_count"`
	MaxCreepCount             int `json:"max_creep_count"`
	ObservedCreepLoss         int `json:"observed_creep_loss"`
	NetCreepCountChange       int `json:"net_creep_count_change"`

	ClosestT                  float64 `json:"closest_t"`
	MinDistanceWorld          float64 `json:"min_distance_world"`
	MeanDistanceWorld         float64 `json:"mean_distance_world"`
	MaxDistanceWorld          float64 `json:"max_distance_world"`
	MaxTargetSampleAgeSeconds float64 `json:"max_target_sample_age_seconds"`

	StartTargetX              float64 `json:"start_target_x"`
	StartTargetY              float64 `json:"start_target_y"`
	EndTargetX                float64 `json:"end_target_x"`
	EndTargetY                float64 `json:"end_target_y"`
	StartWaveX                float64 `json:"start_wave_x"`
	StartWaveY                float64 `json:"start_wave_y"`
	EndWaveX                  float64 `json:"end_wave_x"`
	EndWaveY                  float64 `json:"end_wave_y"`
}

type targetWaveContact struct {
	t             float64
	targetSampleT float64
	targetX       float64
	targetY       float64
	waveX         float64
	waveY         float64
	distanceWorld float64
	creepCount    int
}

// DeriveTargetWaveTaking uses only the target's causal hero samples and M13's
// reconstructed enemy lane-wave samples. Friendly waves are ignored. A period
// is emitted only when at least three consecutive proximity samples exist and
// at least one creep-count decrease is observed while the target is nearby.
func DeriveTargetWaveTaking(tl *MatchTimeline) TargetWaveTakingTimeline {
	out := TargetWaveTakingTimeline{
		Method:               targetWaveTakingMethod,
		ProximityRadiusWorld: targetWaveTakingProximityRadiusWorld,
		SampleMaxAgeSeconds:  targetWaveTakingSampleMaxAgeSeconds,
		MinContactSamples:    targetWaveTakingMinContactSamples,
		MinObservedCreepLoss: targetWaveTakingMinObservedCreepLoss,
		Periods:              []TargetWaveTakingPeriod{},
	}
	if tl == nil || !tl.LaneWaves.Available {
		return out
	}

	target := tl.Players[fmt.Sprintf("%d", tl.TargetPlayerSlot)]
	if target == nil || (target.Team != 2 && target.Team != 3) || len(target.Samples) == 0 {
		return out
	}
	out.Available = true
	enemyTeam := 5 - target.Team

	for _, wave := range tl.LaneWaves.Waves {
		if wave.Team != enemyTeam || len(wave.Samples) == 0 {
			continue
		}

		contacts := make([]targetWaveContact, 0)
		flush := func() {
			if len(contacts) == 0 {
				return
			}
			out.ExposurePeriodsObserved++
			if len(contacts) < targetWaveTakingMinContactSamples {
				out.RejectedTooShort++
				contacts = contacts[:0]
				return
			}

			period := summarizeTargetWaveTakingPeriod(wave, contacts)
			if period.ObservedCreepLoss < targetWaveTakingMinObservedCreepLoss {
				out.RejectedNoDepletion++
				contacts = contacts[:0]
				return
			}
			out.Periods = append(out.Periods, period)
			contacts = contacts[:0]
		}

		for _, waveSample := range wave.Samples {
			targetSample, ok := freshHeroSampleAtOrBefore(target, waveSample.T, targetWaveTakingSampleMaxAgeSeconds)
			if !ok || !targetSample.Alive {
				flush()
				continue
			}

			distanceTimeline := math.Hypot(targetSample.X-waveSample.CenterX, targetSample.Y-waveSample.CenterY)
			if distanceTimeline > targetWaveTakingProximityRadiusTimeline {
				flush()
				continue
			}

			contact := targetWaveContact{
				t:             waveSample.T,
				targetSampleT: targetSample.T,
				targetX:       targetSample.X,
				targetY:       targetSample.Y,
				waveX:         waveSample.CenterX,
				waveY:         waveSample.CenterY,
				distanceWorld: distanceTimeline * 128.0,
				creepCount:    waveSample.CreepCount,
			}
			if len(contacts) > 0 && contact.t-contacts[len(contacts)-1].t > targetWaveTakingMaxContactGapSeconds {
				flush()
			}
			contacts = append(contacts, contact)
		}
		flush()
	}

	sort.SliceStable(out.Periods, func(i, j int) bool {
		if out.Periods[i].StartT != out.Periods[j].StartT {
			return out.Periods[i].StartT < out.Periods[j].StartT
		}
		if out.Periods[i].Lane != out.Periods[j].Lane {
			return laneOrder(out.Periods[i].Lane) < laneOrder(out.Periods[j].Lane)
		}
		return out.Periods[i].WaveID < out.Periods[j].WaveID
	})
	return out
}

func freshHeroSampleAtOrBefore(player *PlayerTimeline, t, maxAge float64) (HeroSample, bool) {
	if player == nil || len(player.Samples) == 0 {
		return HeroSample{}, false
	}
	i := sort.Search(len(player.Samples), func(i int) bool { return player.Samples[i].T > t })
	if i == 0 {
		return HeroSample{}, false
	}
	sample := player.Samples[i-1]
	if t-sample.T > maxAge {
		return HeroSample{}, false
	}
	return sample, true
}

func summarizeTargetWaveTakingPeriod(wave LaneWave, contacts []targetWaveContact) TargetWaveTakingPeriod {
	first := contacts[0]
	last := contacts[len(contacts)-1]
	period := TargetWaveTakingPeriod{
		WaveID:          wave.ID,
		Lane:            wave.Lane,
		EnemyTeam:       wave.Team,
		SpawnT:          wave.SpawnT,
		StartT:          first.t,
		EndT:            last.t,
		DurationSeconds: last.t - first.t,
		ContactSamples:  len(contacts),
		StartCreepCount: first.creepCount,
		EndCreepCount:   last.creepCount,
		MinCreepCount:   first.creepCount,
		MaxCreepCount:   first.creepCount,
		ClosestT:        first.t,
		MinDistanceWorld: first.distanceWorld,
		MaxDistanceWorld: first.distanceWorld,
		StartTargetX:    first.targetX,
		StartTargetY:    first.targetY,
		EndTargetX:      last.targetX,
		EndTargetY:      last.targetY,
		StartWaveX:      first.waveX,
		StartWaveY:      first.waveY,
		EndWaveX:        last.waveX,
		EndWaveY:        last.waveY,
	}

	var distanceSum float64
	previousCount := first.creepCount
	for _, contact := range contacts {
		distanceSum += contact.distanceWorld
		if contact.distanceWorld < period.MinDistanceWorld {
			period.MinDistanceWorld = contact.distanceWorld
			period.ClosestT = contact.t
		}
		if contact.distanceWorld > period.MaxDistanceWorld {
			period.MaxDistanceWorld = contact.distanceWorld
		}
		if contact.creepCount < period.MinCreepCount {
			period.MinCreepCount = contact.creepCount
		}
		if contact.creepCount > period.MaxCreepCount {
			period.MaxCreepCount = contact.creepCount
		}
		if previousCount > contact.creepCount {
			period.ObservedCreepLoss += previousCount - contact.creepCount
		}
		previousCount = contact.creepCount
		age := contact.t - contact.targetSampleT
		if age > period.MaxTargetSampleAgeSeconds {
			period.MaxTargetSampleAgeSeconds = age
		}
	}
	period.MeanDistanceWorld = distanceSum / float64(len(contacts))
	period.NetCreepCountChange = last.creepCount - first.creepCount
	return period
}
