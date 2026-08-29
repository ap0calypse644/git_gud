package timeline

import "sort"

const targetPostWaveOverstayMethod = "m16_post_primary_next_cohort_outcome_v1"

// TargetPostWaveOverstayTimeline is deterministic evidence for the M17
// post-wave-overstay / second-wave-greed detector. It deliberately emits no
// coaching candidate or greed threshold. Point-in-time state is copied only
// from M16's causal snapshots; next-cohort and death fields are explicitly
// retrospective outcome/context evidence.
type TargetPostWaveOverstayTimeline struct {
	Available bool                            `json:"available"`
	Method    string                          `json:"method"`
	Contexts  []TargetPostWaveOverstayContext `json:"contexts"`
}

// TargetPostWaveOverstayContext starts at the end of one accepted M14 primary
// wave-taking interval. ExposureEndT retains M14's raw proximity tail. M17 can
// therefore distinguish "finished the depletion-supported wave and left" from
// "finished it and stayed/advanced" without pretending that either behavior is
// automatically a mistake.
type TargetPostWaveOverstayContext struct {
	WaveID            string  `json:"wave_id"`
	Lane              string  `json:"lane"`
	EnemyTeam         int     `json:"enemy_team"`
	SpawnT            float64 `json:"spawn_t"`
	FirstDepletionT   float64 `json:"first_depletion_t"`
	LastDepletionT    float64 `json:"last_depletion_t"`
	PrimaryEndT       float64 `json:"primary_end_t"`
	ExposureEndT      float64 `json:"exposure_end_t"`
	ObservedCreepLoss int     `json:"observed_creep_loss"`

	EndState         TargetPostWaveState        `json:"end_state"`
	ExposureEndState TargetPostWaveState        `json:"exposure_end_state"`
	PostPrimary      TargetPostWaveChange       `json:"post_primary"`
	NextCohort       TargetPostWaveNextCohort   `json:"next_cohort"`
	Outcome          TargetPostWaveOutcome      `json:"outcome"`
}

// TargetPostWaveState is a compact, causal M16 state summary. Enemy hero replay
// coordinates are intentionally absent. Support is all fresh living allied
// primary heroes, matching M16 semantics, with the nearest distance retained.
type TargetPostWaveState struct {
	T               float64 `json:"t"`
	TargetAvailable bool    `json:"target_available"`
	TargetAlive     bool    `json:"target_alive"`

	LaneProgressWorld                   *float64 `json:"lane_progress_world,omitempty"`
	LaneOffsetWorld                     *float64 `json:"lane_offset_world,omitempty"`
	FriendlyReferenceTier               int      `json:"friendly_reference_tier,omitempty"`
	ForwardOfFriendlyReferenceWorld     *float64 `json:"forward_of_friendly_reference_world,omitempty"`
	EnemyReferenceTier                  int      `json:"enemy_reference_tier,omitempty"`
	ForwardOfEnemyReferenceWorld        *float64 `json:"forward_of_enemy_reference_world,omitempty"`

	SupportAvailable         bool     `json:"support_available"`
	FreshLivingAllies       int      `json:"fresh_living_allies"`
	NearestAllyDistanceWorld *float64 `json:"nearest_ally_distance_world,omitempty"`

	EnemyKnowledgeAvailable bool     `json:"enemy_knowledge_available"`
	EstimatedVisibleEnemies int      `json:"estimated_visible_enemies"`
	LastSeenEnemies         int      `json:"last_seen_enemies"`
	NeverSeenEnemies        int      `json:"never_seen_enemies"`
	MissingEnemies          int      `json:"missing_enemies"`
	MaxLastSeenAgeSeconds   *float64 `json:"max_last_seen_age_seconds,omitempty"`
}

// TargetPostWaveChange is signed evidence across primary EndT -> raw
// ExposureEndT. Positive lane progress means deeper toward the enemy side;
// positive nearest-ally distance means support became farther away. No
// magnitude is treated as a judgment threshold here.
type TargetPostWaveChange struct {
	DurationSeconds                    float64  `json:"duration_seconds"`
	LaneProgressDeltaWorld             *float64 `json:"lane_progress_delta_world,omitempty"`
	FreshLivingAlliesDelta             int      `json:"fresh_living_allies_delta"`
	NearestAllyDistanceDeltaWorld      *float64 `json:"nearest_ally_distance_delta_world,omitempty"`
	EstimatedVisibleEnemiesDelta       int      `json:"estimated_visible_enemies_delta"`
	MissingEnemiesDelta                int      `json:"missing_enemies_delta"`
	MaxLastSeenAgeDeltaSeconds         *float64 `json:"max_last_seen_age_delta_seconds,omitempty"`
}

// TargetPostWaveNextCohort links the immediately following replay-reconstructed
// enemy cohort on the same lane. A later accepted M14 period for that wave is
// retained as evidence that the target actually stayed/re-engaged for another
// depletion-supported wave. This is retrospective context, not player knowledge
// at PrimaryEndT.
type TargetPostWaveNextCohort struct {
	Available bool `json:"available"`

	WaveID string  `json:"wave_id,omitempty"`
	SpawnT float64 `json:"spawn_t,omitempty"`

	TargetTakingObserved          bool     `json:"target_taking_observed"`
	TakingOverlapsPrimaryEnd      bool     `json:"taking_overlaps_primary_end"`
	TakingStartT                  *float64 `json:"taking_start_t,omitempty"`
	TakingEndT                    *float64 `json:"taking_end_t,omitempty"`
	TakingExposureStartT          *float64 `json:"taking_exposure_start_t,omitempty"`
	TakingExposureEndT            *float64 `json:"taking_exposure_end_t,omitempty"`
	SecondsFromPrimaryEndToTaking *float64 `json:"seconds_from_primary_end_to_taking,omitempty"`
}

// TargetPostWaveOutcome is deliberately separated from decision-time state.
// The next target death may help calibration, but death is not required for an
// eventual M17 candidate and must never be treated as information available at
// PrimaryEndT.
type TargetPostWaveOutcome struct {
	NextTargetDeathT             *float64 `json:"next_target_death_t,omitempty"`
	SecondsFromPrimaryEndToDeath *float64 `json:"seconds_from_primary_end_to_death,omitempty"`
}

func DeriveTargetPostWaveOverstay(tl *MatchTimeline) TargetPostWaveOverstayTimeline {
	out := TargetPostWaveOverstayTimeline{
		Method:   targetPostWaveOverstayMethod,
		Contexts: []TargetPostWaveOverstayContext{},
	}
	if tl == nil || !tl.TargetWaveDanger.Available {
		return out
	}
	out.Available = true

	for _, danger := range tl.TargetWaveDanger.Contexts {
		endSnapshot, endOK := targetWaveDangerSnapshotByKind(danger.Snapshots, "end")
		exposureEndSnapshot, exposureEndOK := targetWaveDangerSnapshotByKind(danger.Snapshots, "exposure_end")

		ctx := TargetPostWaveOverstayContext{
			WaveID:            danger.WaveID,
			Lane:              normalizeLaneName(danger.Lane),
			EnemyTeam:         danger.EnemyTeam,
			SpawnT:            danger.SpawnT,
			FirstDepletionT:   danger.FirstDepletionT,
			LastDepletionT:    danger.LastDepletionT,
			PrimaryEndT:       danger.EndT,
			ExposureEndT:      danger.ExposureEndT,
			ObservedCreepLoss: danger.ObservedCreepLoss,
		}
		if endOK {
			ctx.EndState = summarizeTargetPostWaveState(endSnapshot)
		} else {
			ctx.EndState.T = danger.EndT
		}
		if exposureEndOK {
			ctx.ExposureEndState = summarizeTargetPostWaveState(exposureEndSnapshot)
		} else {
			ctx.ExposureEndState.T = danger.ExposureEndT
		}
		ctx.PostPrimary = summarizeTargetPostWaveChange(ctx.EndState, ctx.ExposureEndState, danger.EndT, danger.ExposureEndT)
		ctx.NextCohort = summarizeNextPostWaveCohort(tl, danger)
		ctx.Outcome = summarizePostWaveOutcome(tl, danger.EndT)
		out.Contexts = append(out.Contexts, ctx)
	}

	sort.SliceStable(out.Contexts, func(i, j int) bool {
		if out.Contexts[i].PrimaryEndT != out.Contexts[j].PrimaryEndT {
			return out.Contexts[i].PrimaryEndT < out.Contexts[j].PrimaryEndT
		}
		return out.Contexts[i].WaveID < out.Contexts[j].WaveID
	})
	return out
}

func targetWaveDangerSnapshotByKind(snapshots []TargetWaveDangerSnapshot, kind string) (TargetWaveDangerSnapshot, bool) {
	for _, snapshot := range snapshots {
		if snapshot.Kind == kind {
			return snapshot, true
		}
	}
	return TargetWaveDangerSnapshot{}, false
}

func summarizeTargetPostWaveState(snapshot TargetWaveDangerSnapshot) TargetPostWaveState {
	out := TargetPostWaveState{
		T:               snapshot.T,
		TargetAvailable: snapshot.TargetAvailable,
		TargetAlive:     snapshot.TargetAlive,
		SupportAvailable: snapshot.TargetAvailable,
		FreshLivingAllies: len(snapshot.NearbyAllies),
		EnemyKnowledgeAvailable: len(snapshot.EnemyKnowledge) > 0,
	}

	if snapshot.LaneProgressAvailable {
		out.LaneProgressWorld = postWaveFloat64(snapshot.TargetLaneProgressWorld)
		out.LaneOffsetWorld = postWaveFloat64(snapshot.TargetLaneOffsetWorld)
	}
	if snapshot.FriendlyRetreatReferenceAvailable && snapshot.LaneProgressAvailable {
		out.FriendlyReferenceTier = snapshot.FriendlyRetreatReferenceTier
		out.ForwardOfFriendlyReferenceWorld = postWaveFloat64(snapshot.TargetForwardOfFriendlyReferenceWorld)
	}
	if snapshot.EnemyForwardReferenceAvailable && snapshot.LaneProgressAvailable {
		out.EnemyReferenceTier = snapshot.EnemyForwardReferenceTier
		out.ForwardOfEnemyReferenceWorld = postWaveFloat64(snapshot.TargetForwardOfEnemyReferenceWorld)
	}
	if len(snapshot.NearbyAllies) > 0 {
		out.NearestAllyDistanceWorld = postWaveFloat64(snapshot.NearbyAllies[0].DistanceWorld)
	}

	for _, enemy := range snapshot.EnemyKnowledge {
		switch enemy.Status {
		case "estimated_visible":
			out.EstimatedVisibleEnemies++
		case "last_seen":
			out.LastSeenEnemies++
			out.MissingEnemies++
			if enemy.SecondsSinceSeen != nil && (out.MaxLastSeenAgeSeconds == nil || *enemy.SecondsSinceSeen > *out.MaxLastSeenAgeSeconds) {
				out.MaxLastSeenAgeSeconds = postWaveFloat64(*enemy.SecondsSinceSeen)
			}
		case "never_seen":
			out.NeverSeenEnemies++
			out.MissingEnemies++
		}
	}
	return out
}

func summarizeTargetPostWaveChange(end, exposureEnd TargetPostWaveState, endT, exposureEndT float64) TargetPostWaveChange {
	out := TargetPostWaveChange{
		DurationSeconds:              exposureEndT - endT,
		FreshLivingAlliesDelta:       exposureEnd.FreshLivingAllies - end.FreshLivingAllies,
		EstimatedVisibleEnemiesDelta: exposureEnd.EstimatedVisibleEnemies - end.EstimatedVisibleEnemies,
		MissingEnemiesDelta:          exposureEnd.MissingEnemies - end.MissingEnemies,
	}
	if out.DurationSeconds < 0 {
		out.DurationSeconds = 0
	}
	if end.LaneProgressWorld != nil && exposureEnd.LaneProgressWorld != nil {
		out.LaneProgressDeltaWorld = postWaveFloat64(*exposureEnd.LaneProgressWorld - *end.LaneProgressWorld)
	}
	if end.NearestAllyDistanceWorld != nil && exposureEnd.NearestAllyDistanceWorld != nil {
		out.NearestAllyDistanceDeltaWorld = postWaveFloat64(*exposureEnd.NearestAllyDistanceWorld - *end.NearestAllyDistanceWorld)
	}
	if end.MaxLastSeenAgeSeconds != nil && exposureEnd.MaxLastSeenAgeSeconds != nil {
		out.MaxLastSeenAgeDeltaSeconds = postWaveFloat64(*exposureEnd.MaxLastSeenAgeSeconds - *end.MaxLastSeenAgeSeconds)
	}
	return out
}

func summarizeNextPostWaveCohort(tl *MatchTimeline, danger TargetWaveDangerContext) TargetPostWaveNextCohort {
	wave, ok := nextEnemyLaneWave(tl.LaneWaves.Waves, danger.EnemyTeam, danger.Lane, danger.SpawnT, danger.WaveID)
	if !ok {
		return TargetPostWaveNextCohort{}
	}
	out := TargetPostWaveNextCohort{
		Available: true,
		WaveID:    wave.ID,
		SpawnT:    wave.SpawnT,
	}
	if period, ok := nextTakingPeriodForWave(tl.TargetWaveTaking.Periods, wave.ID, danger.EndT); ok {
		out.TargetTakingObserved = true
		out.TakingOverlapsPrimaryEnd = period.ExposureStartT <= danger.EndT && period.ExposureEndT >= danger.EndT
		out.TakingStartT = postWaveFloat64(period.StartT)
		out.TakingEndT = postWaveFloat64(period.EndT)
		out.TakingExposureStartT = postWaveFloat64(period.ExposureStartT)
		out.TakingExposureEndT = postWaveFloat64(period.ExposureEndT)
		out.SecondsFromPrimaryEndToTaking = postWaveFloat64(period.StartT - danger.EndT)
	}
	return out
}

func nextEnemyLaneWave(waves []LaneWave, enemyTeam int, lane string, spawnT float64, currentWaveID string) (LaneWave, bool) {
	lane = normalizeLaneName(lane)
	var best LaneWave
	found := false
	for _, wave := range waves {
		if wave.ID == currentWaveID || wave.Team != enemyTeam || normalizeLaneName(wave.Lane) != lane || wave.SpawnT <= spawnT {
			continue
		}
		if !found || wave.SpawnT < best.SpawnT || (wave.SpawnT == best.SpawnT && wave.ID < best.ID) {
			best = wave
			found = true
		}
	}
	return best, found
}

func nextTakingPeriodForWave(periods []TargetWaveTakingPeriod, waveID string, primaryEndT float64) (TargetWaveTakingPeriod, bool) {
	var best TargetWaveTakingPeriod
	found := false
	for _, period := range periods {
		if period.WaveID != waveID || period.ExposureEndT < primaryEndT {
			continue
		}
		if !found || period.StartT < best.StartT {
			best = period
			found = true
		}
	}
	return best, found
}

func summarizePostWaveOutcome(tl *MatchTimeline, primaryEndT float64) TargetPostWaveOutcome {
	for _, death := range tl.Deaths {
		if death.VictimSlot == nil || *death.VictimSlot != tl.TargetPlayerSlot || death.T <= primaryEndT {
			continue
		}
		return TargetPostWaveOutcome{
			NextTargetDeathT:             postWaveFloat64(death.T),
			SecondsFromPrimaryEndToDeath: postWaveFloat64(death.T - primaryEndT),
		}
	}
	return TargetPostWaveOutcome{}
}

func postWaveFloat64(v float64) *float64 {
	return &v
}
