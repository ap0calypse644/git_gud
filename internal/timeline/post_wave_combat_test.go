package timeline

import (
	"math"
	"testing"
)

func TestSummarizeTargetPostWaveCombatContextBetweenClearAndPrimaryEnd(t *testing.T) {
	first := 99.7
	fights := []TargetFightContext{
		{
			ObservedStartT:          90,
			ObservedEndT:            105,
			ObservedTimingAvailable: true,
			TargetInvolved:          false,
		},
		{
			ObservedStartT:               99.7,
			ObservedEndT:                 104,
			ObservedTimingAvailable:      true,
			TargetInvolved:               true,
			TargetFirstInvolvementT:      &first,
			TargetFirstInvolvementSource: "damage_received",
		},
	}

	got := summarizeTargetPostWaveCombatContext(fights, 99, 100, 102)
	if got.ObservedFightOverlapAtLastDepletion != 1 {
		t.Fatalf("observed fights at last depletion = %d, want 1", got.ObservedFightOverlapAtLastDepletion)
	}
	if got.ObservedFightOverlapAtPrimaryEnd != 2 {
		t.Fatalf("observed fights at primary end = %d, want 2", got.ObservedFightOverlapAtPrimaryEnd)
	}
	if got.TargetInvolvedFightOverlapAtLastDepletion != 0 {
		t.Fatalf("target-involved fights at last depletion = %d, want 0", got.TargetInvolvedFightOverlapAtLastDepletion)
	}
	if got.TargetInvolvedFightOverlapAtPrimaryEnd != 1 {
		t.Fatalf("target-involved fights at primary end = %d, want 1", got.TargetInvolvedFightOverlapAtPrimaryEnd)
	}
	if got.TargetCombatStartedByLastDepletion {
		t.Fatal("combat beginning after clear must not be classified as started by last depletion")
	}
	if !got.TargetCombatStartedDuringPostClear {
		t.Fatal("expected target combat to start during post-clear interval")
	}
	if !got.TargetCombatStartedByPrimaryEnd {
		t.Fatal("expected target combat to have started by primary end")
	}
	if got.TargetCombatStartedDuringPostPrimary {
		t.Fatal("pre-end target combat must not be classified as starting during post-primary")
	}
	if got.TargetFirstInvolvementT == nil || *got.TargetFirstInvolvementT != 99.7 {
		t.Fatalf("first involvement = %#v, want 99.7", got.TargetFirstInvolvementT)
	}
	if got.SecondsFromLastDepletionToFirstInvolvement == nil || math.Abs(*got.SecondsFromLastDepletionToFirstInvolvement-0.7) > 1e-9 {
		t.Fatalf("clear-relative involvement = %#v, want +0.7", got.SecondsFromLastDepletionToFirstInvolvement)
	}
	if got.SecondsFromPrimaryEndToFirstInvolvement == nil || math.Abs(*got.SecondsFromPrimaryEndToFirstInvolvement+0.3) > 1e-9 {
		t.Fatalf("primary-relative involvement = %#v, want -0.3", got.SecondsFromPrimaryEndToFirstInvolvement)
	}
	if got.TargetFirstInvolvementSource != "damage_received" {
		t.Fatalf("source = %q, want damage_received", got.TargetFirstInvolvementSource)
	}
}

func TestSummarizeTargetPostWaveCombatContextPostEndCombat(t *testing.T) {
	first := 100.9
	fights := []TargetFightContext{{
		ObservedStartT:               100.9,
		ObservedEndT:                 104,
		ObservedTimingAvailable:      true,
		TargetInvolved:               true,
		TargetFirstInvolvementT:      &first,
		TargetFirstInvolvementSource: "damage_received",
	}}

	got := summarizeTargetPostWaveCombatContext(fights, 99, 100, 102)
	if got.ObservedFightOverlapAtPrimaryEnd != 0 {
		t.Fatalf("observed fights at primary end = %d, want 0", got.ObservedFightOverlapAtPrimaryEnd)
	}
	if got.TargetCombatStartedByLastDepletion || got.TargetCombatStartedByPrimaryEnd {
		t.Fatal("post-end target combat must not be classified as already started")
	}
	if !got.TargetCombatStartedDuringPostClear {
		t.Fatal("post-end combat is also post-clear combat")
	}
	if !got.TargetCombatStartedDuringPostPrimary {
		t.Fatal("expected target combat to start during post-primary")
	}
	if got.SecondsFromLastDepletionToFirstInvolvement == nil || math.Abs(*got.SecondsFromLastDepletionToFirstInvolvement-1.9) > 1e-9 {
		t.Fatalf("clear-relative involvement = %#v, want +1.9", got.SecondsFromLastDepletionToFirstInvolvement)
	}
	if got.SecondsFromPrimaryEndToFirstInvolvement == nil || math.Abs(*got.SecondsFromPrimaryEndToFirstInvolvement-0.9) > 1e-9 {
		t.Fatalf("primary-relative involvement = %#v, want +0.9", got.SecondsFromPrimaryEndToFirstInvolvement)
	}
}
