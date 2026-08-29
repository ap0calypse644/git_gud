package timeline

import "testing"

func TestSummarizeTargetPostWaveCombatContextPreEndCombat(t *testing.T) {
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

	got := summarizeTargetPostWaveCombatContext(fights, 100, 102)
	if got.ObservedFightOverlapAtPrimaryEnd != 2 {
		t.Fatalf("observed fights at primary end = %d, want 2", got.ObservedFightOverlapAtPrimaryEnd)
	}
	if got.TargetInvolvedFightOverlapAtPrimaryEnd != 1 {
		t.Fatalf("target-involved fights at primary end = %d, want 1", got.TargetInvolvedFightOverlapAtPrimaryEnd)
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
	if got.SecondsFromPrimaryEndToFirstInvolvement == nil || *got.SecondsFromPrimaryEndToFirstInvolvement != -0.3 {
		t.Fatalf("relative involvement = %#v, want -0.3", got.SecondsFromPrimaryEndToFirstInvolvement)
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

	got := summarizeTargetPostWaveCombatContext(fights, 100, 102)
	if got.ObservedFightOverlapAtPrimaryEnd != 0 {
		t.Fatalf("observed fights at primary end = %d, want 0", got.ObservedFightOverlapAtPrimaryEnd)
	}
	if got.TargetCombatStartedByPrimaryEnd {
		t.Fatal("post-end target combat must not be classified as started by primary end")
	}
	if !got.TargetCombatStartedDuringPostPrimary {
		t.Fatal("expected target combat to start during post-primary")
	}
	if got.SecondsFromPrimaryEndToFirstInvolvement == nil || *got.SecondsFromPrimaryEndToFirstInvolvement != 0.9 {
		t.Fatalf("relative involvement = %#v, want +0.9", got.SecondsFromPrimaryEndToFirstInvolvement)
	}
}
