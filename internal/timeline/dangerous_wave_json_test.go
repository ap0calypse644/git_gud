package timeline

import (
	"encoding/json"
	"testing"
)

func TestTargetWaveDangerSnapshotJSONPreservesAvailableZeroEvidence(t *testing.T) {
	snapshot := TargetWaveDangerSnapshot{
		Kind:                              "first_depletion",
		T:                                 10,
		TargetAvailable:                   true,
		TargetAlive:                       true,
		WaveAvailable:                     true,
		LaneProgressAvailable:             true,
		TargetLaneProgressWorld:           0,
		TargetLaneOffsetWorld:             0,
		WaveLaneProgressAvailable:         true,
		WaveLaneProgressWorld:             0,
		WaveLaneOffsetWorld:               0,
		FriendlyRetreatReferenceAvailable: true,
		FriendlyRetreatReferenceTier:      3,
		FriendlyRetreatReferenceProgressWorld: 0,
		TargetForwardOfFriendlyReferenceWorld: 0,
		WaveForwardOfFriendlyReferenceWorld:   0,
		EnemyForwardReferenceAvailable:         true,
		EnemyForwardReferenceTier:              3,
		EnemyForwardReferenceProgressWorld:     100,
		TargetForwardOfEnemyReferenceWorld:     0,
		WaveForwardOfEnemyReferenceWorld:       0,
	}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	for _, key := range []string{
		"target_sample_t",
		"target_sample_age_seconds",
		"target_x",
		"target_y",
		"wave_sample_t",
		"wave_sample_age_seconds",
		"wave_x",
		"wave_y",
		"creep_count",
		"target_lane_progress_world",
		"target_lane_offset_world",
		"wave_lane_progress_world",
		"wave_lane_offset_world",
		"friendly_retreat_reference_progress_world",
		"target_forward_of_friendly_reference_world",
		"wave_forward_of_friendly_reference_world",
		"target_forward_of_enemy_reference_world",
		"wave_forward_of_enemy_reference_world",
	} {
		value, ok := fields[key]
		if !ok {
			t.Fatalf("available zero field %q was omitted: %s", key, raw)
		}
		if string(value) != "0" {
			t.Fatalf("field %q = %s, want 0", key, value)
		}
	}
}

func TestTargetWaveDangerSnapshotJSONStillOmitsUnavailableZeroEvidence(t *testing.T) {
	snapshot := TargetWaveDangerSnapshot{Kind: "start", T: 10}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	for _, key := range []string{
		"target_lane_progress_world",
		"wave_lane_progress_world",
		"friendly_retreat_reference_progress_world",
		"target_forward_of_friendly_reference_world",
		"enemy_forward_reference_progress_world",
		"target_forward_of_enemy_reference_world",
	} {
		if _, ok := fields[key]; ok {
			t.Fatalf("unavailable field %q should remain omitted: %s", key, raw)
		}
	}
}
