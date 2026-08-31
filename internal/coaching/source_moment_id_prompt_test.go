package coaching

import (
	"strings"
	"testing"
)

func TestCoachingReportSystemPromptKeepsSourceMomentIDsOutOfProse(t *testing.T) {
	for _, required := range []string{
		"source_moment_id is internal bookkeeping only",
		"Use it only in the structured source_moment_ids field",
		"Never mention or quote a source_moment_id in summary, priorities, title, decision_time_facts, retrospective_outcomes, interpretation, alternative, or why_it_matters",
	} {
		if !strings.Contains(coachingReportSystemPrompt, required) {
			t.Fatalf("system prompt missing source-moment ID prose guardrail %q", required)
		}
	}
}
