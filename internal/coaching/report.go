package coaching

import (
	"fmt"
	"sort"
)

const maxReportMoments = 5

// MatchCoachingReport is the structured, user-facing result of reviewing a
// MatchCoachingInput. Identity, timestamps, source detector types, and source
// confidences are filled from deterministic input rather than trusted to the AI.
type MatchCoachingReport struct {
	MatchID    int64                  `json:"match_id"`
	Hero       string                 `json:"hero"`
	Summary    string                 `json:"summary"`
	Priorities []string               `json:"priorities"`
	Moments    []CoachingReportMoment `json:"moments"`
}

// CoachingReportMoment is one prioritized coaching decision. Multiple detector
// moments may be grouped when they describe the same underlying decision.
// DecisionTimeFacts are facts available at or before the decision; later replay
// consequences are kept separately in RetrospectiveOutcomes.
type CoachingReportMoment struct {
	SourceMomentIndexes   []int    `json:"source_moment_indexes"`
	SourceTypes           []string `json:"source_types"`
	SourceConfidences     []string `json:"source_confidences"`
	StartT                float64  `json:"start_t"`
	EndT                  float64  `json:"end_t"`
	Assessment            string   `json:"assessment"`
	Title                 string   `json:"title"`
	DecisionTimeFacts     []string `json:"decision_time_facts"`
	RetrospectiveOutcomes []string `json:"retrospective_outcomes"`
	Interpretation        string   `json:"interpretation"`
	Alternative           string   `json:"alternative"`
	WhyItMatters          string   `json:"why_it_matters"`
}

type modelReportOutput struct {
	Summary    string                      `json:"summary"`
	Priorities []string                    `json:"priorities"`
	Moments    []modelCoachingReportMoment `json:"moments"`
}

type modelCoachingReportMoment struct {
	SourceMomentIDs       []string `json:"source_moment_ids"`
	Assessment            string   `json:"assessment"`
	Title                 string   `json:"title"`
	DecisionTimeFacts     []string `json:"decision_time_facts"`
	RetrospectiveOutcomes []string `json:"retrospective_outcomes"`
	Interpretation        string   `json:"interpretation"`
	Alternative           string   `json:"alternative"`
	WhyItMatters          string   `json:"why_it_matters"`
}

func sourceMomentID(index int, moment CoachingMoment) string {
	return fmt.Sprintf("m%03d_%s_%.3f", index, moment.Type, moment.StartT)
}

func buildMatchCoachingReport(input MatchCoachingInput, modelOutput modelReportOutput) (MatchCoachingReport, error) {
	if len(modelOutput.Moments) > maxReportMoments {
		return MatchCoachingReport{}, fmt.Errorf("model returned %d moments, max %d", len(modelOutput.Moments), maxReportMoments)
	}

	report := MatchCoachingReport{
		MatchID:    input.MatchID,
		Hero:       input.Hero,
		Summary:    modelOutput.Summary,
		Priorities: append([]string(nil), modelOutput.Priorities...),
		Moments:    make([]CoachingReportMoment, 0, len(modelOutput.Moments)),
	}
	usedSources := make(map[int]struct{})

	for i, raw := range modelOutput.Moments {
		indexes, err := normalizeSourceIDs(raw.SourceMomentIDs, input)
		if err != nil {
			return MatchCoachingReport{}, fmt.Errorf("report moment %d: %w", i, err)
		}
		for _, index := range indexes {
			if _, exists := usedSources[index]; exists {
				return MatchCoachingReport{}, fmt.Errorf("report moment %d reuses source moment %d", i, index)
			}
			usedSources[index] = struct{}{}
		}

		startT := input.Moments[indexes[0]].StartT
		endT := input.Moments[indexes[0]].EndT
		types := make([]string, 0, len(indexes))
		confidences := make([]string, 0, len(indexes))
		for _, index := range indexes {
			source := input.Moments[index]
			if source.StartT < startT {
				startT = source.StartT
			}
			if source.EndT > endT {
				endT = source.EndT
			}
			types = append(types, source.Type)
			confidences = append(confidences, source.Confidence)
		}

		report.Moments = append(report.Moments, CoachingReportMoment{
			SourceMomentIndexes:   indexes,
			SourceTypes:           types,
			SourceConfidences:     confidences,
			StartT:                startT,
			EndT:                  endT,
			Assessment:            raw.Assessment,
			Title:                 raw.Title,
			DecisionTimeFacts:     append([]string(nil), raw.DecisionTimeFacts...),
			RetrospectiveOutcomes: append([]string(nil), raw.RetrospectiveOutcomes...),
			Interpretation:        raw.Interpretation,
			Alternative:           raw.Alternative,
			WhyItMatters:          raw.WhyItMatters,
		})
	}
	return report, nil
}

func normalizeSourceIDs(ids []string, input MatchCoachingInput) ([]int, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("source_moment_ids must not be empty")
	}

	known := make(map[string]int, len(input.Moments))
	for index, moment := range input.Moments {
		known[sourceMomentID(index, moment)] = index
	}

	seen := make(map[string]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate source moment id %q", id)
		}
		seen[id] = struct{}{}
		index, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("unknown source moment id %q", id)
		}
		out = append(out, index)
	}
	sort.Ints(out)
	return out, nil
}

func coachingReportJSONSchema() map[string]any {
	stringArray := func(maxItems int) map[string]any {
		schema := map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		}
		if maxItems > 0 {
			schema["maxItems"] = maxItems
		}
		return schema
	}

	moment := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_moment_ids": map[string]any{
				"type":     "array",
				"items":    map[string]any{"type": "string"},
				"minItems": 1,
			},
			"assessment": map[string]any{
				"type": "string",
				"enum": []string{"review", "likely_mistake", "probably_reasonable"},
			},
			"title":                  map[string]any{"type": "string"},
			"decision_time_facts":    stringArray(6),
			"retrospective_outcomes": stringArray(4),
			"interpretation":         map[string]any{"type": "string"},
			"alternative":            map[string]any{"type": "string"},
			"why_it_matters":         map[string]any{"type": "string"},
		},
		"required": []string{
			"source_moment_ids",
			"assessment",
			"title",
			"decision_time_facts",
			"retrospective_outcomes",
			"interpretation",
			"alternative",
			"why_it_matters",
		},
		"additionalProperties": false,
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary":    map[string]any{"type": "string"},
			"priorities": stringArray(3),
			"moments": map[string]any{
				"type":     "array",
				"items":    moment,
				"maxItems": maxReportMoments,
			},
		},
		"required":             []string{"summary", "priorities", "moments"},
		"additionalProperties": false,
	}
}
