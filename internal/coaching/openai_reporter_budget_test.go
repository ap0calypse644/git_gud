package coaching

import (
	"net/http"
	"testing"
)

func TestNewOpenAIReporterUsesExpandedDefaultOutputBudget(t *testing.T) {
	reporter := NewOpenAIReporter("test-key", "test-model", &http.Client{})
	if reporter.MaxOutputTokens != 16000 {
		t.Fatalf("MaxOutputTokens = %d, want 16000", reporter.MaxOutputTokens)
	}
}
