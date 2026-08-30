package coaching

import (
	"strings"
	"testing"
)

func TestCoachingPromptForbidsUnsupportedPlayerIntentLabels(t *testing.T) {
	for _, required := range []string{
		"do not label the player's intent or mental state",
		"panic",
		"Phrase uncertain motivation as a coaching question or omit it",
	} {
		if !strings.Contains(coachingReportSystemPrompt, required) {
			t.Fatalf("coaching prompt missing %q", required)
		}
	}
}
