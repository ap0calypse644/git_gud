package main

import (
	"context"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/coaching"
)

func TestMissingAPIKeyCoachFailsOnlyWhenInvoked(t *testing.T) {
	_, err := (missingAPIKeyCoach{}).Generate(context.Background(), coaching.MatchCoachingInput{MatchID: 42})
	if err == nil {
		t.Fatal("missing API key coach unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("error = %q", err)
	}
}
