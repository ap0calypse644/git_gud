package coaching

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBuildMatchCoachingInputNormalizesAllCurrentCandidateTypes(t *testing.T) {
	tl := coachingFixtureTimeline()
	got := BuildMatchCoachingInput(tl)

	if got.MatchID != 42 {
		t.Fatalf("match_id=%d, want 42", got.MatchID)
	}
	if got.Hero != "slark" {
		t.Fatalf("hero=%q, want slark", got.Hero)
	}
	if len(got.Moments) != 4 {
		t.Fatalf("moments=%d, want 4: %#v", len(got.Moments), got.Moments)
	}

	wantTypes := []string{
		detector.TypeIsolatedDeathCandidate,
		detector.TypePreFightDeathCandidate,
		detector.TypeBadFightJoinCandidate,
		detector.TypeMissedFightCandidate,
	}
	for i, want := range wantTypes {
		if got.Moments[i].Type != want {
			t.Fatalf("moment[%d].type=%q, want %q", i, got.Moments[i].Type, want)
		}
		if got.Moments[i].Confidence != detector.ConfidenceLow {
			t.Fatalf("moment[%d].confidence=%q, want low", i, got.Moments[i].Confidence)
		}
		if got.Moments[i].StartT != got.Moments[i].EndT {
			t.Fatalf("moment[%d] point candidate has span %f..%f", i, got.Moments[i].StartT, got.Moments[i].EndT)
		}
	}

	if _, ok := got.Moments[0].Evidence.(detector.IsolationEvidence); !ok {
		t.Fatalf("isolated evidence type=%T", got.Moments[0].Evidence)
	}
	if _, ok := got.Moments[1].Evidence.(detector.PreFightEvidence); !ok {
		t.Fatalf("pre-fight evidence type=%T", got.Moments[1].Evidence)
	}
	if _, ok := got.Moments[2].Evidence.(detector.BadFightJoinEvidence); !ok {
		t.Fatalf("bad-join evidence type=%T", got.Moments[2].Evidence)
	}
	if _, ok := got.Moments[3].Evidence.(detector.MissedFightEvidence); !ok {
		t.Fatalf("missed-fight evidence type=%T", got.Moments[3].Evidence)
	}
}

func TestMatchCoachingInputJSONDoesNotExposeRawTimeline(t *testing.T) {
	got := BuildMatchCoachingInput(coachingFixtureTimeline())
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal coaching input: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &top); err != nil {
		t.Fatalf("decode coaching input: %v", err)
	}
	if len(top) != 3 || top["match_id"] == nil || top["hero"] == nil || top["moments"] == nil {
		t.Fatalf("unexpected top-level coaching JSON: %s", encoded)
	}

	text := string(encoded)
	for _, forbidden := range []string{
		`"players"`,
		`"samples"`,
		`"knowledge"`,
		`"vision_sources"`,
		`"target_death_contexts"`,
		`"target_fight_contexts"`,
		"777.125",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("coaching input leaked %q: %s", forbidden, text)
		}
	}
}

func TestBuildMatchCoachingInputNilIsExplicitlyEmpty(t *testing.T) {
	got := BuildMatchCoachingInput(nil)
	if got.MatchID != 0 || got.Hero != "" || len(got.Moments) != 0 {
		t.Fatalf("unexpected nil input result: %#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal nil result: %v", err)
	}
	if !strings.Contains(string(encoded), `"moments":[]`) {
		t.Fatalf("empty moments must serialize as []: %s", encoded)
	}
}

func coachingFixtureTimeline() *timeline.MatchTimeline {
	firstInvolvement := 110.0
	joinDelay := 10.0
	joinDeath := 116.0

	return &timeline.MatchTimeline{
		MatchID:          42,
		TargetPlayerSlot: 1,
		Players: map[string]*timeline.PlayerTimeline{
			"1": {
				PlayerSlot: 1,
				HeroName:   "slark",
				Samples: []timeline.HeroSample{
					{T: 30, Alive: false},
					{T: 70, Alive: true},
				},
			},
			"128": {
				PlayerSlot: 128,
				HeroName:   "axe",
				Samples: []timeline.HeroSample{
					{T: 30, X: 777.125, Y: 888.25, Alive: true},
				},
			},
		},
		Fights: []timeline.FightWindow{
			{
				ObservedStartT: 50,
				ObservedEndT:   60,
				Participants:   []int{0, 1, 2, 128, 129, 130},
				Deaths:         2,
				HeroDamage:     5000,
			},
		},
		TargetDeathContexts: []timeline.TargetDeathContext{
			{
				T:              30,
				NearbyAllies:   []timeline.NearbyAlly{},
				EnemyKnowledge: []timeline.EnemyKnowledgeState{},
			},
		},
		TargetFightContexts: []timeline.TargetFightContext{
			{
				ObservedTimingAvailable: true,
				ObservedStartT:          100,
				ObservedEndT:            120,
				Participants:            []int{0, 1, 2, 128, 129, 130},
				Deaths:                  3,
				HeroDamage:              8000,
				TargetInvolved:          true,
				TargetAtStart: timeline.FightTargetState{
					SampleAvailable:       true,
					Alive:                 true,
					DistanceToFightCenter: 10,
				},
				TargetFirstInvolvementT:              &firstInvolvement,
				SecondsToFirstInvolvement:            &joinDelay,
				TargetDeathT:                         &joinDeath,
				AlliedDeathsBeforeTargetInvolvement: []int{0},
				TargetDamageDealt:                    250,
				TargetDamageReceived:                 1400,
			},
			{
				ObservedTimingAvailable: true,
				ObservedStartT:          200,
				ObservedEndT:            215,
				Participants:            []int{0, 2, 3, 128, 129, 130},
				Deaths:                  2,
				HeroDamage:              7000,
				TargetInvolved:          false,
				TargetAtStart: timeline.FightTargetState{
					SampleAvailable:       true,
					Alive:                 true,
					DistanceToFightCenter: 10,
				},
				TeammatesAtStart: []timeline.FightTeammateState{
					{PlayerSlot: 0, SampleAvailable: true, Alive: true},
					{PlayerSlot: 2, SampleAvailable: true, Alive: true},
					{PlayerSlot: 3, SampleAvailable: true, Alive: true},
					{PlayerSlot: 4, SampleAvailable: true, Alive: true},
				},
			},
		},
	}
}
