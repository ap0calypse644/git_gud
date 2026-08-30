package coaching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIReporterUsesLowReasoningEffort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		reasoning, ok := request["reasoning"].(map[string]any)
		if !ok {
			t.Fatalf("reasoning=%#v", request["reasoning"])
		}
		if got := reasoning["effort"]; got != "low" {
			t.Fatalf("reasoning.effort=%v, want low", got)
		}
		if got := request["max_output_tokens"]; got != float64(defaultReportMaxOutputTokens) {
			t.Fatalf("max_output_tokens=%v, want %d", got, defaultReportMaxOutputTokens)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message",
				"content": []any{map[string]any{
					"type":    "refusal",
					"refusal": "test complete",
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
