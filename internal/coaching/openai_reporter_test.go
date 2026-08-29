package coaching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIReporterGenerateUsesOnlyCompactInputAndStructuredSchema(t *testing.T) {
	input := MatchCoachingInput{
		MatchID: 123,
		Hero:    "slark",
		Moments: []CoachingMoment{{
			Type:       "post_wave_overstay_candidate",
			StartT:     196,
			EndT:       202,
			Confidence: "low",
			Evidence: PostWaveOverstayReviewEvidence{
				WaveID:                             "3:150:bottom",
				Lane:                               "bottom",
				LastDepletionT:                     196,
				ExposureEndT:                       202,
				PostClearDurationSeconds:           6,
				PostClearLaneProgressDeltaWorld:    1444,
				TargetCombatStartedDuringPostClear: true,
				SecondsFromClearToFirstInvolvement: 1.9,
			},
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path=%q, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization=%q", got)
		}

		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request["model"] != "test-model" {
			t.Errorf("model=%v", request["model"])
		}
		if store, ok := request["store"].(bool); !ok || store {
			t.Errorf("store=%v, want false", request["store"])
		}
		text, ok := request["text"].(map[string]any)
		if !ok {
			t.Fatalf("text config type=%T", request["text"])
		}
		format, ok := text["format"].(map[string]any)
		if !ok || format["type"] != "json_schema" || format["strict"] != true {
			t.Fatalf("format=%#v", text["format"])
		}

		messages, ok := request["input"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("input messages=%#v", request["input"])
		}
		userMessage, ok := messages[1].(map[string]any)
		if !ok {
			t.Fatalf("user message=%#v", messages[1])
		}
		userContent, ok := userMessage["content"].(string)
		if !ok {
			t.Fatalf("user content=%#v", userMessage["content"])
		}
		for _, required := range []string{"\"match_id\":123", "post_wave_overstay_candidate", "3:150:bottom"} {
			if !strings.Contains(userContent, required) {
				t.Errorf("user payload missing %q: %s", required, userContent)
			}
		}
		for _, forbidden := range []string{"target_wave_danger", "target_post_wave_overstay", "\"players\"", "777.125", "888.25"} {
			if strings.Contains(userContent, forbidden) {
				t.Errorf("user payload leaked %q: %s", forbidden, userContent)
			}
		}

		structured, err := json.Marshal(modelReportOutput{
			Summary:    "One lane decision deserves review.",
			Priorities: []string{"Disengage after the wave when the next step is unclear."},
			Moments: []modelCoachingReportMoment{{
				SourceMomentIndexes: []int{0},
				Assessment:          "likely_mistake",
				Title:               "Stayed after the wave was cleared",
				DeterministicFacts:  []string{"The post-clear exposure continued for six seconds."},
				Interpretation:      "Continuing forward likely increased the chance of being forced into combat.",
				Alternative:         "Reset after the clear instead of continuing the lane exposure.",
				WhyItMatters:        "A clean reset preserves farm tempo and reduces unnecessary risk.",
			}},
		})
		if err != nil {
			t.Fatalf("marshal structured response: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message",
				"content": []any{map[string]any{
					"type": "output_text",
					"text": string(structured),
				}},
			}},
		})
	}))
	defer server.Close()

	reporter := NewOpenAIReporter("test-key", "test-model", server.Client())
	reporter.BaseURL = server.URL
	report, err := reporter.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if report.MatchID != input.MatchID || report.Hero != input.Hero {
		t.Fatalf("identity=%d/%q", report.MatchID, report.Hero)
	}
	if len(report.Moments) != 1 {
		t.Fatalf("moments=%d, want 1", len(report.Moments))
	}
	if report.Moments[0].StartT != 196 || report.Moments[0].EndT != 202 {
		t.Fatalf("derived time=%v..%v", report.Moments[0].StartT, report.Moments[0].EndT)
	}
	if len(report.Moments[0].SourceConfidences) != 1 || report.Moments[0].SourceConfidences[0] != "low" {
		t.Fatalf("source confidence=%v", report.Moments[0].SourceConfidences)
	}
}

func TestOpenAIReporterGenerateRejectsRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message",
				"content": []any{map[string]any{
					"type":    "refusal",
					"refusal": "cannot comply",
				}},
			}},
		})
	}))
	defer server.Close()

	reporter := NewOpenAIReporter("test-key", "test-model", server.Client())
	reporter.BaseURL = server.URL
	_, err := reporter.Generate(context.Background(), MatchCoachingInput{})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("err=%v, want refusal error", err)
	}
}
