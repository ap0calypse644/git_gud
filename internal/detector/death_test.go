package detector

import (
	"math"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestIsolatedDeathRejectsTeamfightScaleDeath(t *testing.T) {
	ctx := timeline.TargetDeathContext{
		T: 100,
		Fight: &timeline.DeathFightContext{
			Participants: []int{0, 1, 2, 128, 129, 130},
			Deaths:       2,
			HeroDamage:   5000,
		},
	}
	got := assessIsolation(ctx)
	if got.Candidate {
		t.Fatalf("teamfight-scale death must not be isolated candidate: %#v", got)
	}
	if !got.Evidence.TeamfightScale {
		t.Fatalf("expected teamfight-scale evidence")
	}
}

func TestIsolatedDeathCandidateForSmallFightWithoutImmediateSupport(t *testing.T) {
	age := 40.0
	ctx := timeline.TargetDeathContext{
		T: 100,
		Fight: &timeline.DeathFightContext{
			Participants: []int{1, 128, 129},
			Deaths:       1,
			HeroDamage:   1400,
		},
		NearbyAllies: []timeline.NearbyAlly{
			{PlayerSlot: 0, Distance: IsolationSupportRadiusTimeline + 0.1},
		},
		DamageReceivedLast10s: 1200,
		DamageDealtLast10s:    100,
		EnemyKnowledge: []timeline.EnemyKnowledgeState{
			{PlayerSlot: 128, Status: "estimated_visible"},
			{PlayerSlot: 129, Status: "last_seen", SecondsSinceSeen: &age},
		},
	}

	got := assessIsolation(ctx)
	if !got.Candidate {
		t.Fatalf("small unsupported fight should be candidate: %#v", got)
	}
	if got.Evidence.NearbyAlliesWithinSupport != 0 {
		t.Fatalf("ally outside support radius counted as immediate support: %#v", got.Evidence)
	}
	if got.Evidence.EstimatedVisibleEnemies != 1 || len(got.Evidence.MissingEnemies) != 1 {
		t.Fatalf("unexpected enemy knowledge evidence: %#v", got.Evidence)
	}
}

func TestIsolatedDeathSupportRadiusBoundaryIsInclusive(t *testing.T) {
	ctx := timeline.TargetDeathContext{
		T: 100,
		Fight: &timeline.DeathFightContext{Participants: []int{1, 128}},
		NearbyAllies: []timeline.NearbyAlly{
			{PlayerSlot: 0, Distance: IsolationSupportRadiusTimeline},
		},
	}
	got := assessIsolation(ctx)
	if got.Candidate {
		t.Fatalf("ally exactly on support boundary should suppress candidate: %#v", got)
	}
	if got.Evidence.NearbyAlliesWithinSupport != 1 {
		t.Fatalf("boundary ally not counted: %#v", got.Evidence)
	}
}

func TestAnalyzeDeathsEmitsLowConfidenceNormalizedCandidate(t *testing.T) {
	tl := &timeline.MatchTimeline{
		MatchID:          7,
		TargetPlayerSlot: 1,
		TargetDeathContexts: []timeline.TargetDeathContext{
			{T: 50, Fight: &timeline.DeathFightContext{Participants: []int{1, 128}}},
		},
		Players: map[string]*timeline.PlayerTimeline{
			"1": {PlayerSlot: 1},
		},
	}
	got := AnalyzeDeaths(tl)
	if len(got.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1: %#v", len(got.Candidates), got)
	}
	candidate := got.Candidates[0]
	if candidate.Type != TypeIsolatedDeathCandidate || candidate.Confidence != ConfidenceLow || candidate.Isolation == nil {
		t.Fatalf("unexpected normalized candidate: %#v", candidate)
	}
}

func TestPreFightDeathCandidateWhenTeamfightStartsBeforeRespawn(t *testing.T) {
	ctx := timeline.TargetDeathContext{
		T:     10,
		Fight: &timeline.DeathFightContext{Participants: []int{1, 128, 129}},
	}
	tl := &timeline.MatchTimeline{
		TargetPlayerSlot: 1,
		Players: map[string]*timeline.PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				Samples: []timeline.HeroSample{
					{T: 10.2, Alive: false},
					{T: 29.9, Alive: false},
					{T: 30.1, Alive: true},
				},
			},
		},
		Fights: []timeline.FightWindow{
			{ObservedStartT: 20, ObservedEndT: 28, Participants: []int{0, 2, 3, 128, 129, 130}, Deaths: 2, HeroDamage: 6000},
		},
	}
	got := assessPreFight(tl, tl.Players["1"], ctx)
	if !got.Candidate || !got.Evidence.TargetDeadAtTeamfightStart {
		t.Fatalf("expected pre-fight candidate: %#v", got)
	}
	if got.Evidence.RespawnT == nil || math.Abs(*got.Evidence.RespawnT-30.1) > 1e-9 {
		t.Fatalf("unexpected respawn: %#v", got.Evidence.RespawnT)
	}
	if got.Evidence.SecondsUntilTeamfight == nil || *got.Evidence.SecondsUntilTeamfight != 10 {
		t.Fatalf("unexpected seconds-until-teamfight: %#v", got.Evidence.SecondsUntilTeamfight)
	}
}

func TestPreFightDeathRejectedWhenRespawnComesFirst(t *testing.T) {
	ctx := timeline.TargetDeathContext{T: 10, Fight: &timeline.DeathFightContext{Participants: []int{1, 128}}}
	target := &timeline.PlayerTimeline{PlayerSlot: 1, Samples: []timeline.HeroSample{
		{T: 10.2, Alive: false},
		{T: 15, Alive: true},
	}}
	tl := &timeline.MatchTimeline{
		Fights: []timeline.FightWindow{{ObservedStartT: 20, Participants: []int{0, 2, 3, 128, 129, 130}}},
	}
	got := assessPreFight(tl, target, ctx)
	if got.Candidate || got.Evidence.TargetDeadAtTeamfightStart {
		t.Fatalf("respawn-before-fight must not be pre-fight candidate: %#v", got)
	}
}

func TestPreFightDeathRejectedWhenCurrentDeathAlreadyInTeamfight(t *testing.T) {
	ctx := timeline.TargetDeathContext{
		T:     10,
		Fight: &timeline.DeathFightContext{Participants: []int{0, 1, 2, 128, 129, 130}},
	}
	target := &timeline.PlayerTimeline{PlayerSlot: 1, Samples: []timeline.HeroSample{{T: 10.1, Alive: false}}}
	tl := &timeline.MatchTimeline{
		Fights: []timeline.FightWindow{{ObservedStartT: 20, Participants: []int{0, 2, 3, 128, 129, 130}}},
	}
	got := assessPreFight(tl, target, ctx)
	if got.Candidate {
		t.Fatalf("death already inside teamfight-scale engagement must not be pre-fight candidate: %#v", got)
	}
}

func TestPreFightRequiresObservedUnpaddedTiming(t *testing.T) {
	ctx := timeline.TargetDeathContext{T: 10, Fight: &timeline.DeathFightContext{Participants: []int{1, 128}}}
	target := &timeline.PlayerTimeline{PlayerSlot: 1, Samples: []timeline.HeroSample{{T: 10.1, Alive: false}}}
	tl := &timeline.MatchTimeline{
		Fights: []timeline.FightWindow{{StartT: 20, EndT: 30, Participants: []int{0, 2, 3, 128, 129, 130}}},
	}
	got := assessPreFight(tl, target, ctx)
	if got.Candidate || got.Evidence.ObservedFightTimingAvailable {
		t.Fatalf("padded timing must not be silently used as observed start: %#v", got)
	}
}

func TestAnalyzeDeathsHandlesNilTimeline(t *testing.T) {
	got := AnalyzeDeaths(nil)
	if len(got.Assessments) != 0 || len(got.Candidates) != 0 {
		t.Fatalf("unexpected nil-timeline output: %#v", got)
	}
}
