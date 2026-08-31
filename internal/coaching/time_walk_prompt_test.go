package coaching

import (
	"strings"
	"testing"
)

func TestCoachingReportSystemPromptCalibratesTimeWalkEvidence(t *testing.T) {
	for _, required := range []string{
		"latest replay sample observed at or before the cast",
		"Never subtract target_damage_received_before_cast",
		"do not establish that Time Walk was available earlier or later",
		"Never infer cooldown, mana, destination, Reverse Time Walk availability, or an exact amount healed",
		"can support a probably_reasonable recovery cast rather than a mistake",
		"Observed Time Walk casts elsewhere in the match do not establish Time Walk availability at another decision",
		"do not present Time Walk as an available alternative unless that moment explicitly establishes availability",
		"if available",
	} {
		if !strings.Contains(coachingReportSystemPrompt, required) {
			t.Fatalf("system prompt missing Time Walk calibration %q", required)
		}
	}
}
