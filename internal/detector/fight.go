package detector

import (
	"sort"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const (
	// These are deliberately transparent first-pass calibration thresholds.
	// A candidate is only a review target, never a coaching verdict.
	BadFightJoinMinDelaySeconds       = 5.0
	BadFightJoinMaxSurvivalSeconds   = 10.0
	MissedFightReviewRadiusWorld     = 3000.0
	MissedFightReviewRadiusTimeline  = MissedFightReviewRadiusWorld / worldUnitsPerTimelineCoord
	MissedFightMinAlliedParticipants = 3
)

const (
	TypeBadFightJoinCandidate = "bad_fight_join_candidate"
	TypeMissedFightCandidate  = "missed_fight_candidate"
)

// FightAnalysis keeps an assessment for every M8 fight context so calibration
// can inspect both emitted candidates and obvious false negatives.
type FightAnalysis struct {
	MatchID     int64             `json:"match_id"`
	Assessments []FightAssessment `json:"assessments"`
	Candidates  []FightCandidate  `json:"candidates"`
}

type FightCandidate struct {
	Type         string                `json:"type"`
	T            float64               `json:"t"`
	Confidence   string                `json:"confidence"`
	BadFightJoin *BadFightJoinEvidence `json:"bad_fight_join,omitempty"`
	MissedFight  *MissedFightEvidence  `json:"missed_fight,omitempty"`
}

type FightAssessment struct {
	ObservedStartT float64                `json:"observed_start_t"`
	BadFightJoin   BadFightJoinAssessment `json:"bad_fight_join"`
	MissedFight    MissedFightAssessment  `json:"missed_fight"`
}

type BadFightJoinAssessment struct {
	Candidate bool                 `json:"candidate"`
	Evidence  BadFightJoinEvidence `json:"evidence"`
}

type BadFightJoinEvidence struct {
	ObservedTimingAvailable       bool     `json:"observed_timing_available"`
	TeamfightScale                bool     `json:"teamfight_scale"`
	FightParticipants             int      `json:"fight_participants"`
	FightDeaths                   int      `json:"fight_deaths"`
	FightHeroDamage               int64    `json:"fight_hero_damage"`
	TargetInvolved                bool     `json:"target_involved"`
	TargetStartSampleAvailable    bool     `json:"target_start_sample_available"`
	TargetAliveAtStart            bool     `json:"target_alive_at_start"`
	TargetDistanceToCenter        *float64 `json:"target_distance_to_center,omitempty"`
	FirstInvolvementT             *float64 `json:"first_involvement_t,omitempty"`
	SecondsToFirstInvolvement     *float64 `json:"seconds_to_first_involvement,omitempty"`
	MinDelaySeconds               float64  `json:"min_delay_seconds"`
	AlliedDeathsBeforeInvolvement []int    `json:"allied_deaths_before_involvement"`
	TargetDeathT                  *float64 `json:"target_death_t,omitempty"`
	SecondsFromInvolvementToDeath *float64 `json:"seconds_from_involvement_to_death,omitempty"`
	MaxPostJoinSurvivalSeconds    float64  `json:"max_post_join_survival_seconds"`
	TargetDamageDealt             int64    `json:"target_damage_dealt"`
	TargetDamageReceived          int64    `json:"target_damage_received"`
}

type MissedFightAssessment struct {
	Candidate bool                `json:"candidate"`
	Evidence  MissedFightEvidence `json:"evidence"`
}

type MissedFightEvidence struct {
	ObservedTimingAvailable    bool     `json:"observed_timing_available"`
	TeamfightScale             bool     `json:"teamfight_scale"`
	FightParticipants          int      `json:"fight_participants"`
	FightDeaths                int      `json:"fight_deaths"`
	FightHeroDamage            int64    `json:"fight_hero_damage"`
	TargetInvolved             bool     `json:"target_involved"`
	TargetStartSampleAvailable bool     `json:"target_start_sample_available"`
	TargetAliveAtStart         bool     `json:"target_alive_at_start"`
	TargetDistanceToCenter     *float64 `json:"target_distance_to_center,omitempty"`
	ReviewRadiusWorld          float64  `json:"review_radius_world"`
	ReviewRadiusTimeline       float64  `json:"review_radius_timeline"`
	AlliedParticipantSlots     []int    `json:"allied_participant_slots"`
	AlliedParticipants         int      `json:"allied_participants"`
	MinAlliedParticipants      int      `json:"min_allied_participants"`
	AliveTeammatesAtStart      int      `json:"alive_teammates_at_start"`
	EstimatedVisibleEnemies    int      `json:"estimated_visible_enemies"`
}

// AnalyzeFights emits low-confidence candidate judgments from the validated M8
// target-fight contexts. It never queries enemy PlayerTimeline positions; enemy
// context comes only from EnemyKnowledgeAtStart.
func AnalyzeFights(tl *timeline.MatchTimeline) FightAnalysis {
	out := FightAnalysis{Assessments: []FightAssessment{}, Candidates: []FightCandidate{}}
	if tl == nil {
		return out
	}
	out.MatchID = tl.MatchID

	contexts := tl.TargetFightContexts
	if len(contexts) == 0 {
		contexts = timeline.DeriveTargetFightContexts(tl)
	}

	for _, ctx := range contexts {
		badJoin := assessBadFightJoin(ctx)
		missed := assessMissedFight(ctx)
		out.Assessments = append(out.Assessments, FightAssessment{
			ObservedStartT: ctx.ObservedStartT,
			BadFightJoin:   badJoin,
			MissedFight:    missed,
		})

		if badJoin.Candidate {
			evidence := badJoin.Evidence
			t := ctx.ObservedStartT
			if ctx.TargetFirstInvolvementT != nil {
				t = *ctx.TargetFirstInvolvementT
			}
			out.Candidates = append(out.Candidates, FightCandidate{
				Type:         TypeBadFightJoinCandidate,
				T:            t,
				Confidence:   ConfidenceLow,
				BadFightJoin: &evidence,
			})
		}
		if missed.Candidate {
			evidence := missed.Evidence
			out.Candidates = append(out.Candidates, FightCandidate{
				Type:        TypeMissedFightCandidate,
				T:           ctx.ObservedStartT,
				Confidence:  ConfidenceLow,
				MissedFight: &evidence,
			})
		}
	}

	sort.SliceStable(out.Assessments, func(i, j int) bool {
		return out.Assessments[i].ObservedStartT < out.Assessments[j].ObservedStartT
	})
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].T == out.Candidates[j].T {
			return out.Candidates[i].Type < out.Candidates[j].Type
		}
		return out.Candidates[i].T < out.Candidates[j].T
	})
	return out
}

func assessBadFightJoin(ctx timeline.TargetFightContext) BadFightJoinAssessment {
	evidence := BadFightJoinEvidence{
		ObservedTimingAvailable:       ctx.ObservedTimingAvailable,
		TeamfightScale:                len(ctx.Participants) >= TeamfightMinParticipants,
		FightParticipants:             len(ctx.Participants),
		FightDeaths:                   ctx.Deaths,
		FightHeroDamage:               ctx.HeroDamage,
		TargetInvolved:                ctx.TargetInvolved,
		TargetStartSampleAvailable:    ctx.TargetAtStart.SampleAvailable,
		TargetAliveAtStart:            ctx.TargetAtStart.SampleAvailable && ctx.TargetAtStart.Alive,
		FirstInvolvementT:             copyFloat64Ptr(ctx.TargetFirstInvolvementT),
		SecondsToFirstInvolvement:     copyFloat64Ptr(ctx.SecondsToFirstInvolvement),
		MinDelaySeconds:               BadFightJoinMinDelaySeconds,
		AlliedDeathsBeforeInvolvement: append([]int{}, ctx.AlliedDeathsBeforeTargetInvolvement...),
		TargetDeathT:                  copyFloat64Ptr(ctx.TargetDeathT),
		MaxPostJoinSurvivalSeconds:    BadFightJoinMaxSurvivalSeconds,
		TargetDamageDealt:             ctx.TargetDamageDealt,
		TargetDamageReceived:          ctx.TargetDamageReceived,
	}
	if ctx.TargetAtStart.SampleAvailable {
		d := ctx.TargetAtStart.DistanceToFightCenter
		evidence.TargetDistanceToCenter = &d
	}
	if ctx.TargetFirstInvolvementT != nil && ctx.TargetDeathT != nil && *ctx.TargetDeathT >= *ctx.TargetFirstInvolvementT {
		d := *ctx.TargetDeathT - *ctx.TargetFirstInvolvementT
		evidence.SecondsFromInvolvementToDeath = &d
	}

	candidate := evidence.ObservedTimingAvailable &&
		evidence.TeamfightScale &&
		evidence.TargetInvolved &&
		evidence.TargetStartSampleAvailable &&
		evidence.TargetAliveAtStart &&
		evidence.SecondsToFirstInvolvement != nil &&
		*evidence.SecondsToFirstInvolvement >= BadFightJoinMinDelaySeconds &&
		len(evidence.AlliedDeathsBeforeInvolvement) > 0 &&
		evidence.SecondsFromInvolvementToDeath != nil &&
		*evidence.SecondsFromInvolvementToDeath <= BadFightJoinMaxSurvivalSeconds

	return BadFightJoinAssessment{Candidate: candidate, Evidence: evidence}
}

func assessMissedFight(ctx timeline.TargetFightContext) MissedFightAssessment {
	alliedSlots := alliedParticipantSlots(ctx)
	evidence := MissedFightEvidence{
		ObservedTimingAvailable:    ctx.ObservedTimingAvailable,
		TeamfightScale:             len(ctx.Participants) >= TeamfightMinParticipants,
		FightParticipants:          len(ctx.Participants),
		FightDeaths:                ctx.Deaths,
		FightHeroDamage:            ctx.HeroDamage,
		TargetInvolved:             ctx.TargetInvolved,
		TargetStartSampleAvailable: ctx.TargetAtStart.SampleAvailable,
		TargetAliveAtStart:         ctx.TargetAtStart.SampleAvailable && ctx.TargetAtStart.Alive,
		ReviewRadiusWorld:          MissedFightReviewRadiusWorld,
		ReviewRadiusTimeline:       MissedFightReviewRadiusTimeline,
		AlliedParticipantSlots:     alliedSlots,
		AlliedParticipants:         len(alliedSlots),
		MinAlliedParticipants:      MissedFightMinAlliedParticipants,
		AliveTeammatesAtStart:      aliveTeammatesAtFightStart(ctx),
		EstimatedVisibleEnemies:    estimatedVisibleEnemiesAtFightStart(ctx),
	}
	if ctx.TargetAtStart.SampleAvailable {
		d := ctx.TargetAtStart.DistanceToFightCenter
		evidence.TargetDistanceToCenter = &d
	}

	// Deliberately narrow: this detector does not reason about TP availability,
	// cooldowns, lane responsibility, objectives, or whether joining was actually
	// correct. It only surfaces a nearby, consequential teamfight the alive
	// target did not enter for later review.
	candidate := evidence.ObservedTimingAvailable &&
		evidence.TeamfightScale &&
		!evidence.TargetInvolved &&
		evidence.FightDeaths > 0 &&
		evidence.TargetStartSampleAvailable &&
		evidence.TargetAliveAtStart &&
		evidence.TargetDistanceToCenter != nil &&
		*evidence.TargetDistanceToCenter <= MissedFightReviewRadiusTimeline &&
		evidence.AlliedParticipants >= MissedFightMinAlliedParticipants

	return MissedFightAssessment{Candidate: candidate, Evidence: evidence}
}

func alliedParticipantSlots(ctx timeline.TargetFightContext) []int {
	teammates := make(map[int]struct{}, len(ctx.TeammatesAtStart))
	for _, teammate := range ctx.TeammatesAtStart {
		teammates[teammate.PlayerSlot] = struct{}{}
	}
	out := make([]int, 0, 4)
	for _, slot := range ctx.Participants {
		if _, ok := teammates[slot]; ok {
			out = append(out, slot)
		}
	}
	sort.Ints(out)
	return out
}

func aliveTeammatesAtFightStart(ctx timeline.TargetFightContext) int {
	count := 0
	for _, teammate := range ctx.TeammatesAtStart {
		if teammate.SampleAvailable && teammate.Alive {
			count++
		}
	}
	return count
}

func estimatedVisibleEnemiesAtFightStart(ctx timeline.TargetFightContext) int {
	count := 0
	for _, enemy := range ctx.EnemyKnowledgeAtStart {
		if enemy.Status == "estimated_visible" {
			count++
		}
	}
	return count
}

func copyFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}
