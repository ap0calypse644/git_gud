package timeline

import "encoding/json"

// MarshalJSON keeps availability booleans authoritative for M16 evidence.
// Several valid measurements can legitimately be numeric zero (for example a
// target projected at friendly T3, or a signed distance exactly on a tower
// reference). The struct tags retain omitempty for compact in-memory defaults,
// but when the corresponding evidence is available we must serialize those
// zero values instead of making them indistinguishable from missing data.
func (s TargetWaveDangerSnapshot) MarshalJSON() ([]byte, error) {
	type snapshotAlias TargetWaveDangerSnapshot

	raw, err := json.Marshal(snapshotAlias(s))
	if err != nil {
		return nil, err
	}

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}

	put := func(key string, value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		fields[key] = encoded
		return nil
	}

	if s.TargetAvailable {
		for key, value := range map[string]any{
			"target_sample_t":           s.TargetSampleT,
			"target_sample_age_seconds": s.TargetSampleAge,
			"target_x":                  s.TargetX,
			"target_y":                  s.TargetY,
			"target_alive":              s.TargetAlive,
		} {
			if err := put(key, value); err != nil {
				return nil, err
			}
		}
	}

	if s.WaveAvailable {
		for key, value := range map[string]any{
			"wave_sample_t":           s.WaveSampleT,
			"wave_sample_age_seconds": s.WaveSampleAge,
			"wave_x":                  s.WaveX,
			"wave_y":                  s.WaveY,
			"creep_count":             s.CreepCount,
		} {
			if err := put(key, value); err != nil {
				return nil, err
			}
		}
	}

	if s.LaneProgressAvailable {
		if err := put("target_lane_progress_world", s.TargetLaneProgressWorld); err != nil {
			return nil, err
		}
		if err := put("target_lane_offset_world", s.TargetLaneOffsetWorld); err != nil {
			return nil, err
		}
	}
	if s.WaveLaneProgressAvailable {
		if err := put("wave_lane_progress_world", s.WaveLaneProgressWorld); err != nil {
			return nil, err
		}
		if err := put("wave_lane_offset_world", s.WaveLaneOffsetWorld); err != nil {
			return nil, err
		}
	}

	if s.FriendlyRetreatReferenceAvailable {
		if err := put("friendly_retreat_reference_progress_world", s.FriendlyRetreatReferenceProgressWorld); err != nil {
			return nil, err
		}
		if s.LaneProgressAvailable {
			if err := put("target_forward_of_friendly_reference_world", s.TargetForwardOfFriendlyReferenceWorld); err != nil {
				return nil, err
			}
		}
		if s.WaveLaneProgressAvailable {
			if err := put("wave_forward_of_friendly_reference_world", s.WaveForwardOfFriendlyReferenceWorld); err != nil {
				return nil, err
			}
		}
	}

	if s.EnemyForwardReferenceAvailable {
		if err := put("enemy_forward_reference_progress_world", s.EnemyForwardReferenceProgressWorld); err != nil {
			return nil, err
		}
		if s.LaneProgressAvailable {
			if err := put("target_forward_of_enemy_reference_world", s.TargetForwardOfEnemyReferenceWorld); err != nil {
				return nil, err
			}
		}
		if s.WaveLaneProgressAvailable {
			if err := put("wave_forward_of_enemy_reference_world", s.WaveForwardOfEnemyReferenceWorld); err != nil {
				return nil, err
			}
		}
	}

	return json.Marshal(fields)
}
