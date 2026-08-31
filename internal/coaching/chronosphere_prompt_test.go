package coaching

import (
	"strings"
	"testing"
)

func TestCoachingReportSystemPromptSeparatesChronospherePreCastAndFollowupEvidence(t *testing.T) {
	for _, required := range []string{
		"recent_enemy_interactors_before_cast and recent_allied_teammates_interacting_with_same_enemies_before_cast are the only Chronosphere follow-up interaction counts that describe pre-cast context",
		"target_enemy_heroes_damaged_in_followup",
		"post-cast retrospective evidence only",
		"Never copy, substitute, or paraphrase a follow-up count as a pre-cast count",
		"does not identify which heroes were caught by Chronosphere or establish cast placement quality",
		"damage-relationship aggregates only",
		"do not establish hero proximity or visibility",
		"never as nearby, visible, or positional context",
	} {
		if !strings.Contains(coachingReportSystemPrompt, required) {
			t.Fatalf("system prompt missing Chronosphere temporal calibration %q", required)
		}
	}
}
