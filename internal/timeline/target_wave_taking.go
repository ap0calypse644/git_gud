package timeline

import (
	"fmt"
	"math"
	"sort"
)

const (
	targetWaveTakingMethod                  = "enemy_lane_wave_proximity_and_depletion_death_aware_v3"
	targetWaveTakingProximityRadiusWorld    = 1600.0
	targetWaveTakingProximityRadiusTimeline = targetWaveTakingProximityRadiusWorld / 128.0
	targetWaveTakingSampleMaxAgeSeconds     = 1.5
	targetWaveTakingMaxContactGapSeconds    = 1.5
	targetWaveTakingMinContactSamples       = 3
	targetWaveTakingMinObservedCreepLoss    = 1
	targetWaveTakingDepletionContextSamples = 1
)

// TargetWaveTakingTimeline is deterministic derived context for periods where
// the configured player stayed close to an enemy lane wave while that wave was
// observed losing creeps. It is intentionally a conservative *candidate* for
// taking a wave, not proof that the player personally last-hit or damaged any
// creep. Later danger/overstay detectors may consume these periods as context.
type TargetWaveTakingTimeline struct {
	Available                     bool                     `json:"available"`
	Method                        string                   `json:"method"`
	ProximityRadiusWorld          float64                  `json:"proximity_radius_world"`
	SampleMaxAgeSeconds           float64                  `json:"sample_max_age_seconds"`
	MinContactSamples             int                      `json:"min_contact_samples"`
	MinObservedCreepLoss          int                      `json:"min_observed_creep_loss"`
	DepletionContextSamples       int                      `json:"depletion_context_samples"`
	ExposurePeriodsObserved       int                      `json:"exposure_periods_observed"`
	RejectedTooShort              int                      `json:"rejected_too_short"`
	RejectedNoDepletion           int                      `json:"rejected_no_depletion"`
	LeadingContactSamplesTrimmed  int                      `json:"leading_contact_samples_trimmed"`
	TrailingContactSamplesTrimmed int                      `json:"trailing_contact_samples_trimmed"`
	Periods                       []TargetWaveTakingPeriod `json:"periods"`
}

// TargetWaveTakingPeriod summarizes the depletion-supported portion of one
// contiguous proximity exposure to a reconstructed enemy lane wave.
// ExposureStartT/ExposureEndT retain the full raw proximity run for audit. The
// primary StartT/EndT are bounded around the first/last observed creep-count
// decrease so a surviving straggler cannot keep a wave-taking period open for
// tens of seconds after depletion stopped.
type TargetWaveTakingPeriod struct {
	WaveID                 string  `json:"wave_id"`
	Lane                   string  `json:"lane"`
	EnemyTeam              int     `json:"enemy_team"`
	SpawnT                 float64 `json:"spawn_t"`
	StartT                 float64 `json:"start_t"`
	EndT                   float64 `json:"end_t"`
	DurationSeconds        float64 `json:"duration_seconds"`
	ContactSamples         int     `json:"contact_samples"`
	ExposureStartT         float64 `json:"exposure_start_t"`
	ExposureEndT           float64 `json:"exposure_end_t"`
	ExposureContactSamples int     `json:"exposure_contact_samples"`
	FirstDepletionT        float64 `json:"first_depletion_t"`
	LastDepletionT         float64 `json:"last_depletion_t"`

	StartCreepCount     int `json:"start_creep_count"`
	EndCreepCount       int `json:"end_creep_count"`
	MinCreepCount       int `json:"min_creep_count"`
	MaxCreepCount       int `json:"max_creep_count"`
	ObservedCreepLoss   int `json:"observed_creep_loss"`
	NetCreepCountChange int `json:"net_creep_count_change"`

	ClosestT                  float64 `json:"closest_t"`
	MinDistanceWorld          float64 `json:"min_distance_world"`
	MeanDistanceWorld         float64 `json:"mean_distance_world"`
	MaxDistanceWorld          float64 `json:"max_distance_world"`
	MaxTargetSampleAgeSeconds float64 `json:"max_target_sample_age_seconds"`

	StartTargetX float64 `json:"start_target_x"`
	StartTargetY float64 `json:"start_target_y"`
	EndTargetX   float64 `json:"end_target_x"`
	EndTargetY   float64 `json:"end_target_y"`
	StartWaveX   float64 `json:"start_wave_x"`
	StartWaveY   float64 `json:"start_wave_y"`
	EndWaveX     float64 `json:"end_wave_x"`
	EndWaveY     float64 `json:"end_wave_y"`
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

// DeriveTargetWaveTaking uses only the target's causal hero samples, exact
// replay death events, and M13's reconstructed enemy lane-wave samples.
// Friendly waves are ignored. A raw exposure must contain at least three
// consecutive proximity samples and at least one creep-count decrease.
// Accepted output is then trimmed around the observed depletion activity while
// retaining the raw exposure bounds.
func DeriveTargetWaveTaking(tl *MatchTimeline) TargetWaveTakingTimeline {
	out := TargetWaveTakingTimeline{
		Method:                  targetWaveTakingMethod,
		ProximityRadiusWorld:    targetWaveTakingProximityRadiusWorld,
		SampleMaxAgeSeconds:     targetWaveTakingSampleMaxAgeSeconds,
		MinContactSamples:       targetWaveTakingMinContactSamples,
		MinObservedCreepLoss:    targetWaveTakingMinObservedCreepLoss,
		DepletionContextSamples: targetWaveTakingDepletionContextSamples,
		Periods:                 []TargetWaveTakingPeriod{},
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

			bounded, firstDepletionT, lastDepletionT, leadingTrimmed, trailingTrimmed, ok := depletionBoundedContacts(contacts)
			if !ok {
				out.RejectedNoDepletion++
				contacts = contacts[:0]
				return
			}
			out.LeadingContactSamplesTrimmed += leadingTrimmed
			out.TrailingContactSamplesTrimmed += trailingTrimmed

			period := summarizeTargetWaveTakingPeriod(wave, bounded)
			if period.ObservedCreepLoss < targetWaveTakingMinObservedCreepLoss {
				out.RejectedNoDepletion++
				contacts = contacts[:0]
				return
			}
			period.ExposureStartT = contacts[0].t
			period.ExposureEndT = contacts[len(contacts)-1].t
			period.ExposureContactSamples = len(contacts)
			period.FirstDepletionT = firstDepletionT
			period.LastDepletionT = lastDepletionT
			out.Periods = append(out.Periods, period)
			contacts = contacts[:0]
		}

		for _, waveSample := range wave.Samples {
			targetSample, ok := freshLivingHeroSampleAtOrBefore(tl, target, waveSample.T, targetWaveTakingSampleMaxAgeSeconds)
			if !ok {
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

// depletionBoundedContacts keeps a small amount of proximity context around
// the first and last actual count decrease. It never widens beyond the raw
// exposure. If necessary it expands by nearby contacts only to preserve the
// configured minimum sample evidence.
func depletionBoundedContacts(contacts []targetWaveContact) ([]targetWaveContact, float64, float64, int, int, bool) {
	if len(contacts) < 2 {
		return nil, 0, 0, 0, 0, false
	}
	firstLossIndex, lastLossIndex := -1, -1
	for i := 1; i < len(contacts); i++ {
		if contacts[i-1].creepCount > contacts[i].creepCount {
			if firstLossIndex < 0 {
				firstLossIndex = i
			}
			lastLossIndex = i
		}
	}
	if firstLossIndex < 0 {
		return nil, 0, 0, 0, 0, false
	}

	start := firstLossIndex - 1 - targetWaveTakingDepletionContextSamples
	if start < 0 {
		start = 0
	}
	end := lastLossIndex + targetWaveTakingDepletionContextSamples
	if end >= len(contacts) {
		end = len(contacts) - 1
	}
	for end-start+1 < targetWaveTakingMinContactSamples && start > 0 {
		start--
	}
	for end-start+1 < targetWaveTakingMinContactSamples && end < len(contacts)-1 {
		end++
	}

	bounded := contacts[start : end+1]
	return bounded,
		contacts[firstLossIndex].t,
		contacts[lastLossIndex].t,
		start,
		len(contacts) - 1 - end,
		true
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

// freshLivingHeroSampleAtOrBefore upgrades a fresh causal sample with exact
// replay death evidence. This closes the sub-second gap where the latest 1 Hz
// hero sample can still say alive after an exact death event. A later alive
// sample naturally represents a respawn because only deaths at or after that
// sample and at or before t invalidate it.
func freshLivingHeroSampleAtOrBefore(tl *MatchTimeline, player *PlayerTimeline, t, maxAge float64) (HeroSample, bool) {
	sample, ok := freshHeroSampleAtOrBefore(player, t, maxAge)
	if !ok || !sample.Alive || tl == nil || player == nil {
		return HeroSample{}, false
	}
	for _, death := range tl.Deaths {
		if death.VictimSlot == nil || *death.VictimSlot != player.PlayerSlot {
			continue
		}
		if death.T >= sample.T && death.T <= t {
			return HeroSample{}, false
		}
	}
	return sample, true
}

func summarizeTargetWaveTakingPeriod(wave LaneWave, contacts []targetWaveContact) TargetWaveTakingPeriod {
	first := contacts[0]
	last := contacts[len(contacts)-1]
	period := TargetWaveTakingPeriod{
		WaveID:           wave.ID,
		Lane:             wave.Lane,
		EnemyTeam:        wave.Team,
		SpawnT:           wave.SpawnT,
		StartT:           first.t,
		EndT:             last.t,
		DurationSeconds:  last.t - first.t,
		ContactSamples:   len(contacts),
		StartCreepCount:  first.creepCount,
		EndCreepCount:    last.creepCount,
		MinCreepCount:    first.creepCount,
		MaxCreepCount:    first.creepCount,
		ClosestT:         first.t,
		MinDistanceWorld: first.distanceWorld,
		MaxDistanceWorld: first.distanceWorld,
		StartTargetX:     first.targetX,
		StartTargetY:     first.targetY,
		EndTargetX:       last.targetX,
		EndTargetY:       last.targetY,
		StartWaveX:       first.waveX,
		StartWaveY:       first.waveY,
		EndWaveX:         last.waveX,
		EndWaveY:         last.waveY,
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
