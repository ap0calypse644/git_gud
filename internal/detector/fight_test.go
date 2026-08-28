package detector

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBadFightJoinCandidateForLateDoomedJoinAfterAllyDeath(t *testing.T) {
	first := 110.0
	delay := 10.0
	death := 116.0
	ctx := timeline.TargetFightContext{
		ObservedTimingAvailable: true,
		ObservedStartT:          100,
		Participants:            []int{0, 1, 2, 128, 129, 130},
		Deaths:                  3,
		HeroDamage:              8000,
		TargetInvolved:          true,
		TargetAtStart: timeline.FightTargetState{
			SampleAvailable:       true,
			Alive:                 true,
			DistanceToFightCenter: 18,
		},
		TargetFirstInvolvementT:            &first,
		SecondsToFirstInvolvement:          &delay,
		TargetDeathT:                       &death,
		AlliedDeathsBeforeTargetInvolvement: []int{0},
		TargetDamageDealt:                  250,
		TargetDamageReceived:               1400,
	}

	got := assessBadFightJoin(ctx)
	if !got.Candidate {
		t.Fatalf("expected bad-fight-join candidate: %#v", got)
	}
	if got.Evidence.SecondsFromInvolvementToDeath == nil || *got.Evidence.SecondsFromInvolvementToDeath != 6 {
		t.Fatalf("unexpected post-join survival: %#v", got.Evidence.SecondsFromInvolvementToDeath)
	}
}

func TestBadFightJoinThresholdBoundariesAreInclusive(t *testing.T) {
	first := 105.0
	delay := BadFightJoinMinDelaySeconds
	death := first + BadFightJoinMaxSurvivalSeconds
	ctx := timeline.TargetFightContext{
		ObservedTimingAvailable: true,
		ObservedStartT:          100,
		Participants:            []int{0, 1, 2, 128, 129, 130},
		Deaths:                  2,
		TargetInvolved:          true,
		TargetAtStart:           timeline.FightTargetState{SampleAvailable: true, Alive: true},
		TargetFirstInvolvementT: &first,
		SecondsToFirstInvolvement: &delay,
		TargetDeathT:            &death,
		AlliedDeathsBeforeTargetInvolvement: []int{0},
	}
	if got := assessBadFightJoin(ctx); !got.Candidate {
		t.Fatalf("exact delay/survival boundaries should be candidates: %#v", got)
	}
}

func TestBadFightJoinRejectedWithoutPriorAllyDeathOrTargetDeath(t *testing.T) {
	first := 110.0
	delay := 10.0
	base := timeline.TargetFightContext{
		ObservedTimingAvailable: true,
		ObservedStartT:          100,
		Participants:            []int{0, 1, 2, 128, 129, 130},
		TargetInvolved:          true,
		TargetAtStart:           timeline.FightTargetState{SampleAvailable: true, Alive: true},
		TargetFirstInvolvementT: &first,
		SecondsToFirstInvolvement: &delay,
	}
	if got := assessBadFightJoin(base); got.Candidate {
		t.Fatalf("surviving join without prior ally death must not be candidate: %#v", got)
	}
	death := 115.0
	base.TargetDeathT = &death
	if got := assessBadFightJoin(base); got.Candidate {
		t.Fatalf("join with no allied death before involvement must not be candidate: %#v", got)
	}
}

func TestMissedFightCandidateForNearbyAliveTargetWithThreeAlliesCommitted(t *testing.T) {
	ctx := missedFightContext(MissedFightReviewRadiusTimeline)
	got := assessMissedFight(ctx)
	if !got.Candidate {
		t.Fatalf("expected nearby missed-fight candidate: %#v", got)
	}
	if got.Evidence.AlliedParticipants != 3 {
		t.Fatalf("allied participants=%d, want 3", got.Evidence.AlliedParticipants)
	}
	if math.Abs(*got.Evidence.TargetDistanceToCenter-MissedFightReviewRadiusTimeline) > 1e-9 {
		t.Fatalf("unexpected distance: %#v", got.Evidence.TargetDistanceToCenter)
	}
}

func TestMissedFightReviewRadiusBoundaryRejectsOutside(t *testing.T) {
	ctx := missedFightContext(MissedFightReviewRadiusTimeline + 0.001)
	if got := assessMissedFight(ctx); got.Candidate {
		t.Fatalf("target outside review radius must not be candidate: %#v", got)
	}
}

func TestMissedFightRejectsDeadUnavailableOrTooFewAlliedParticipants(t *testing.T) {
	ctx := missedFightContext(10)
	ctx.TargetAtStart.Alive = false
	if got := assessMissedFight(ctx); got.Candidate {
		t.Fatalf("dead target must not be missed-fight candidate: %#v", got)
	}

	ctx = missedFightContext(10)
	ctx.TargetAtStart.SampleAvailable = false
	if got := assessMissedFight(ctx); got.Candidate {
		t.Fatalf("missing target sample must not be candidate: %#v", got)
	}

	ctx = missedFightContext(10)
	ctx.Participants = []int{0, 2, 128, 129, 130, 131}
	if got := assessMissedFight(ctx); got.Candidate {
		t.Fatalf("fewer than three allied participants must not be candidate: %#v", got)
	}
}

func TestAnalyzeFightsEmitsLowConfidenceFightCandidates(t *testing.T) {
	first := 110.0
	delay := 10.0
	death := 116.0
	badJoin := timeline.TargetFightContext{
		ObservedTimingAvailable: true,
		ObservedStartT:          100,
		Participants:            []int{0, 1, 2, 128, 129, 130},
		TargetInvolved:          true,
		TargetAtStart:           timeline.FightTargetState{SampleAvailable: true, Alive: true},
		TargetFirstInvolvementT: &first,
		SecondsToFirstInvolvement: &delay,
		TargetDeathT:            &death,
		AlliedDeathsBeforeTargetInvolvement: []int{0},
	}
	missed := missedFightContext(10)
	missed.ObservedStartT = 200

	tl := &timeline.MatchTimeline{
		MatchID:             99,
		TargetFightContexts: []timeline.TargetFightContext{badJoin, missed},
	}
	got := AnalyzeFights(tl)
	if len(got.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2: %#v", len(got.Candidates), got)
	}
	if got.Candidates[0].Type != TypeBadFightJoinCandidate || got.Candidates[0].Confidence != ConfidenceLow || got.Candidates[0].BadFightJoin == nil {
		t.Fatalf("unexpected bad-join candidate: %#v", got.Candidates[0])
	}
	if got.Candidates[1].Type != TypeMissedFightCandidate || got.Candidates[1].Confidence != ConfidenceLow || got.Candidates[1].MissedFight == nil {
		t.Fatalf("unexpected missed-fight candidate: %#v", got.Candidates[1])
	}
}

func TestAnalyzeFightsHandlesNilTimeline(t *testing.T) {
	got := AnalyzeFights(nil)
	if len(got.Assessments) != 0 || len(got.Candidates) != 0 {
		t.Fatalf("unexpected nil-timeline output: %#v", got)
	}
}

func missedFightContext(distance float64) timeline.TargetFightContext {
	return timeline.TargetFightContext{
		ObservedTimingAvailable: true,
		ObservedStartT:          100,
		Participants:            []int{0, 2, 3, 128, 129, 130},
		Deaths:                  2,
		HeroDamage:              7000,
		TargetInvolved:          false,
		TargetAtStart: timeline.FightTargetState{
			SampleAvailable:       true,
			Alive:                 true,
			DistanceToFightCenter: distance,
		},
		TeammatesAtStart: []timeline.FightTeammateState{
			{PlayerSlot: 0, SampleAvailable: true, Alive: true},
			{PlayerSlot: 2, SampleAvailable: true, Alive: true},
			{PlayerSlot: 3, SampleAvailable: true, Alive: true},
			{PlayerSlot: 4, SampleAvailable: true, Alive: true},
		},
		EnemyKnowledgeAtStart: []timeline.EnemyKnowledgeState{
			{PlayerSlot: 128, Status: "estimated_visible"},
		},
	}
}
