package patterns

import (
	"testing"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/detector"
)

func TestStoreExcludesUncalibratedKeyAbilityCandidateFamilies(t *testing.T) {
	store := NewStore(t.TempDir(), 20)
	input := coaching.MatchCoachingInput{
		MatchID: 8973449904,
		Hero:    "faceless_void",
		Moments: []coaching.CoachingMoment{
			{Type: detector.TypeKeyAbilityUseReviewCandidate, StartT: 100, EndT: 100, Confidence: detector.ConfidenceLow},
			{Type: detector.TypeActiveDamageReflectInteractionCandidate, StartT: 100, EndT: 100, Confidence: detector.ConfidenceLow},
			{Type: detector.TypeIsolatedDeathCandidate, StartT: 120, EndT: 120, Confidence: detector.ConfidenceLow},
		},
	}
	if _, err := store.Record(input); err != nil {
		t.Fatal(err)
	}

	history, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Matches) != 1 {
		t.Fatalf("matches = %d", len(history.Matches))
	}
	if got, want := len(history.Matches[0].Observations), 1; got != want {
		t.Fatalf("observations = %#v", history.Matches[0].Observations)
	}
	if history.Matches[0].Observations[0].Type != detector.TypeIsolatedDeathCandidate {
		t.Fatalf("recorded unexpected type %q", history.Matches[0].Observations[0].Type)
	}
	if len(history.Patterns) != 1 || history.Patterns[0].Type != detector.TypeIsolatedDeathCandidate {
		t.Fatalf("patterns = %#v", history.Patterns)
	}
}
