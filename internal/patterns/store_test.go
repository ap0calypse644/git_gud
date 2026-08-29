package patterns

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/coaching"
)

func TestStoreAggregatesDistinctMatchRecurrenceAndContext(t *testing.T) {
	store := NewStore(t.TempDir(), 20)
	postWave := func(matchID int64, hero, lane string, tSeconds float64) coaching.MatchCoachingInput {
		return coaching.MatchCoachingInput{
			MatchID: matchID,
			Hero:    hero,
			Moments: []coaching.CoachingMoment{{
				Type:       "post_wave_overstay_candidate",
				StartT:     tSeconds,
				EndT:       tSeconds + 5,
				Confidence: "low",
				Evidence: coaching.PostWaveOverstayReviewEvidence{
					Lane: lane,
				},
			}},
		}
	}

	if _, err := store.Record(postWave(100, "slark", "bottom", 300)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record(postWave(101, "faceless_void", "top", 1300)); err != nil {
		t.Fatal(err)
	}

	history, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if history.RecentMatchesConsidered != 2 || len(history.Patterns) != 1 {
		t.Fatalf("history=%#v", history)
	}
	pattern := history.Patterns[0]
	if pattern.Type != "post_wave_overstay_candidate" || pattern.MatchesWithPattern != 2 || pattern.Occurrences != 2 || !pattern.Recurring {
		t.Fatalf("pattern=%#v", pattern)
	}
	if math.Abs(pattern.MatchRate-1) > 1e-9 {
		t.Fatalf("match rate=%f", pattern.MatchRate)
	}
	if pattern.Heroes["slark"] != 1 || pattern.Heroes["faceless_void"] != 1 {
		t.Fatalf("heroes=%#v", pattern.Heroes)
	}
	if pattern.GamePhases["early"] != 1 || pattern.GamePhases["mid"] != 1 {
		t.Fatalf("phases=%#v", pattern.GamePhases)
	}
	if pattern.Lanes["bottom"] != 1 || pattern.Lanes["top"] != 1 {
		t.Fatalf("lanes=%#v", pattern.Lanes)
	}
}

func TestStoreUpsertsMatchWithoutDoubleCounting(t *testing.T) {
	store := NewStore(t.TempDir(), 20)
	input := coaching.MatchCoachingInput{
		MatchID: 200,
		Hero:    "slark",
		Moments: []coaching.CoachingMoment{{Type: "isolated_death_candidate", StartT: 900, EndT: 900, Confidence: "low"}},
	}
	if _, err := store.Record(input); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record(input); err != nil {
		t.Fatal(err)
	}

	history, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Matches) != 1 || len(history.Patterns) != 1 {
		t.Fatalf("history=%#v", history)
	}
	if history.Patterns[0].MatchesWithPattern != 1 || history.Patterns[0].Occurrences != 1 || history.Patterns[0].Recurring {
		t.Fatalf("pattern=%#v", history.Patterns[0])
	}
}

func TestStoreSummaryUsesOnlyMostRecentNMatches(t *testing.T) {
	store := NewStore(t.TempDir(), 2)
	inputs := []coaching.MatchCoachingInput{
		{MatchID: 300, Hero: "slark", Moments: []coaching.CoachingMoment{{Type: "isolated_death_candidate", StartT: 100}}},
		{MatchID: 301, Hero: "slark", Moments: []coaching.CoachingMoment{{Type: "missed_fight_candidate", StartT: 700}}},
		{MatchID: 302, Hero: "slark", Moments: []coaching.CoachingMoment{{Type: "missed_fight_candidate", StartT: 1700}}},
	}
	for _, input := range inputs {
		if _, err := store.Record(input); err != nil {
			t.Fatal(err)
		}
	}

	history, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Matches) != 3 || history.RecentMatchesConsidered != 2 {
		t.Fatalf("history=%#v", history)
	}
	if len(history.Patterns) != 1 || history.Patterns[0].Type != "missed_fight_candidate" {
		t.Fatalf("patterns=%#v", history.Patterns)
	}
	if history.Patterns[0].MatchesWithPattern != 2 || !history.Patterns[0].Recurring {
		t.Fatalf("pattern=%#v", history.Patterns[0])
	}
}
