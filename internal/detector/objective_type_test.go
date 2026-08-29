package detector

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestAnalyzeObjectivesEmitsReviewCandidateType(t *testing.T) {
	tl, ctx := baseObjectiveFixture()
	tl.TargetPostFightObjectives = timeline.PostFightObjectiveTimeline{
		Available: true,
		Contexts:  []timeline.PostFightObjectiveContext{ctx},
	}

	analysis := AnalyzeObjectives(tl)
	if len(analysis.Candidates) != 1 {
		t.Fatalf("candidates = %#v, want one", analysis.Candidates)
	}
	if analysis.Candidates[0].Type != "post_fight_conversion_review_candidate" {
		t.Fatalf("candidate type = %q, want post_fight_conversion_review_candidate", analysis.Candidates[0].Type)
	}
}
