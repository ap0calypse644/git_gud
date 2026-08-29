package coaching

import "testing"

func TestBuildMatchCoachingReportDerivesSourceMetadata(t *testing.T) {
	input := MatchCoachingInput{
		MatchID: 99,
		Hero:    "slark",
		Moments: []CoachingMoment{
			{Type: "isolated_death_candidate", StartT: 100, EndT: 100, Confidence: "low"},
			{Type: "pre_fight_death_candidate", StartT: 100, EndT: 105, Confidence: "medium"},
		},
	}
	modelOutput := modelReportOutput{
		Summary:    "One important decision overlapped two review signals.",
		Priorities: []string{"Avoid being unavailable before the next fight."},
		Moments: []modelCoachingReportMoment{{
			SourceMomentIndexes: []int{1, 0},
			Assessment:          "likely_mistake",
			Title:               "Death before the next fight",
			DeterministicFacts:  []string{"Two review signals share the same decision window."},
			Interpretation:      "This likely reduced your ability to contest the following fight.",
			Alternative:         "Back away earlier and preserve your life for the next engagement.",
			WhyItMatters:        "Being alive preserves map pressure and fight participation.",
		}},
	}

	report, err := buildMatchCoachingReport(input, modelOutput)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.MatchID != input.MatchID || report.Hero != input.Hero {
		t.Fatalf("identity=%d/%q, want %d/%q", report.MatchID, report.Hero, input.MatchID, input.Hero)
	}
	if len(report.Moments) != 1 {
		t.Fatalf("moments=%d, want 1", len(report.Moments))
	}
	moment := report.Moments[0]
	if len(moment.SourceMomentIndexes) != 2 || moment.SourceMomentIndexes[0] != 0 || moment.SourceMomentIndexes[1] != 1 {
		t.Fatalf("source indexes=%v, want [0 1]", moment.SourceMomentIndexes)
	}
	if moment.StartT != 100 || moment.EndT != 105 {
		t.Fatalf("derived span=%v..%v, want 100..105", moment.StartT, moment.EndT)
	}
	if len(moment.SourceTypes) != 2 || moment.SourceTypes[0] != input.Moments[0].Type || moment.SourceTypes[1] != input.Moments[1].Type {
		t.Fatalf("source types=%v", moment.SourceTypes)
	}
	if len(moment.SourceConfidences) != 2 || moment.SourceConfidences[0] != "low" || moment.SourceConfidences[1] != "medium" {
		t.Fatalf("source confidences=%v", moment.SourceConfidences)
	}
}

func TestBuildMatchCoachingReportRejectsReusedSourceMoment(t *testing.T) {
	input := MatchCoachingInput{
		Moments: []CoachingMoment{{Type: "x", StartT: 1, EndT: 1, Confidence: "low"}},
	}
	modelOutput := modelReportOutput{
		Moments: []modelCoachingReportMoment{
			{SourceMomentIndexes: []int{0}},
			{SourceMomentIndexes: []int{0}},
		},
	}
	if _, err := buildMatchCoachingReport(input, modelOutput); err == nil {
		t.Fatal("reused source moment unexpectedly accepted")
	}
}

func TestNormalizeSourceIndexesRejectsInvalidIndexes(t *testing.T) {
	for _, indexes := range [][]int{{}, {-1}, {2}, {1, 1}} {
		if _, err := normalizeSourceIndexes(indexes, 2); err == nil {
			t.Fatalf("indexes %v unexpectedly accepted", indexes)
		}
	}
}
