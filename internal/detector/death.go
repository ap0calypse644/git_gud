package detector

import (
	"sort"
	"strconv"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

const (
	// 1500 world units is an intentionally transparent first-pass immediate
	// support radius. It is a candidate threshold, not a claim that allies
	// outside it can never help.
	IsolationSupportRadiusWorld    = 1500.0
	worldUnitsPerTimelineCoord     = 128.0
	IsolationSupportRadiusTimeline = IsolationSupportRadiusWorld / worldUnitsPerTimelineCoord

	// Six distinct participants is used as a conservative first-pass signal
	// that an engagement is teamfight-scale rather than a small pickoff. This
	// must be calibrated across multiple matches before confidence is raised.
	TeamfightMinParticipants = 6
)

const (
	TypeIsolatedDeathCandidate = "isolated_death_candidate"
	TypePreFightDeathCandidate = "pre_fight_death_candidate"
	ConfidenceLow              = "low"
)

// Analysis contains both all-death assessments and emitted candidates. Keeping
// assessments makes false negatives inspectable during detector calibration;
// Candidates is the normalized stream later coaching layers can consume.
type Analysis struct {
	MatchID     int64             `json:"match_id"`
	Assessments []DeathAssessment `json:"assessments"`
	Candidates  []Candidate       `json:"candidates"`
}

type DeathAssessment struct {
	T         float64              `json:"t"`
	Isolation IsolationAssessment  `json:"isolation"`
	PreFight  PreFightAssessment   `json:"pre_fight"`
}

type Candidate struct {
	Type       string                 `json:"type"`
	T          float64                `json:"t"`
	Confidence string                 `json:"confidence"`
	Isolation  *IsolationEvidence     `json:"isolation,omitempty"`
	PreFight   *PreFightEvidence      `json:"pre_fight,omitempty"`
}

type IsolationAssessment struct {
	Candidate bool              `json:"candidate"`
	Evidence  IsolationEvidence `json:"evidence"`
}

type IsolationEvidence struct {
	TeamfightScale             bool                   `json:"teamfight_scale"`
	FightParticipants          int                    `json:"fight_participants"`
	FightDeaths                int                    `json:"fight_deaths"`
	FightHeroDamage            int64                  `json:"fight_hero_damage"`
	SupportRadiusWorld         float64                `json:"support_radius_world"`
	SupportRadiusTimeline      float64                `json:"support_radius_timeline"`
	NearbyAlliesWithinSupport  int                    `json:"nearby_allies_within_support"`
	NearestAllyDistance        *float64               `json:"nearest_ally_distance,omitempty"`
	DamageReceivedLast10s      int64                  `json:"damage_received_last_10s"`
	DamageDealtLast10s         int64                  `json:"damage_dealt_last_10s"`
	EstimatedVisibleEnemies    int                    `json:"estimated_visible_enemies"`
	MissingEnemies             []MissingEnemyEvidence `json:"missing_enemies"`
}

type MissingEnemyEvidence struct {
	PlayerSlot       int      `json:"player_slot"`
	Status           string   `json:"status"`
	SecondsSinceSeen *float64 `json:"seconds_since_seen,omitempty"`
}

type PreFightAssessment struct {
	Candidate bool             `json:"candidate"`
	Evidence  PreFightEvidence `json:"evidence"`
}

type PreFightEvidence struct {
	ObservedFightTimingAvailable bool     `json:"observed_fight_timing_available"`
	CurrentFightTeamfightScale   bool     `json:"current_fight_teamfight_scale"`
	TargetDeathConfirmed         bool     `json:"target_death_confirmed"`
	RespawnT                     *float64 `json:"respawn_t,omitempty"`
	NextTeamfightStartT          *float64 `json:"next_teamfight_start_t,omitempty"`
	SecondsUntilTeamfight        *float64 `json:"seconds_until_teamfight,omitempty"`
	TargetDeadAtTeamfightStart   bool     `json:"target_dead_at_teamfight_start"`
	NextFightParticipants        int      `json:"next_fight_participants,omitempty"`
	NextFightDeaths              int      `json:"next_fight_deaths,omitempty"`
	NextFightHeroDamage          int64    `json:"next_fight_hero_damage,omitempty"`
}

// AnalyzeDeaths emits deliberately low-confidence candidate judgments from M6
// target-death contexts. No detector reads enemy PlayerTimeline positions;
// enemy information comes only from the causal EnemyKnowledgeState already
// embedded in TargetDeathContext.
func AnalyzeDeaths(tl *timeline.MatchTimeline) Analysis {
	out := Analysis{Assessments: []DeathAssessment{}, Candidates: []Candidate{}}
	if tl == nil {
		return out
	}
	out.MatchID = tl.MatchID

	contexts := tl.TargetDeathContexts
	if len(contexts) == 0 {
		contexts = timeline.DeriveTargetDeathContexts(tl)
	}
	target := targetPlayer(tl)

	for _, ctx := range contexts {
		isolation := assessIsolation(ctx)
		preFight := assessPreFight(tl, target, ctx)
		out.Assessments = append(out.Assessments, DeathAssessment{
			T:         ctx.T,
			Isolation: isolation,
			PreFight:  preFight,
		})

		if isolation.Candidate {
			evidence := isolation.Evidence
			out.Candidates = append(out.Candidates, Candidate{
				Type:       TypeIsolatedDeathCandidate,
				T:          ctx.T,
				Confidence: ConfidenceLow,
				Isolation:  &evidence,
			})
		}
		if preFight.Candidate {
			evidence := preFight.Evidence
			out.Candidates = append(out.Candidates, Candidate{
				Type:       TypePreFightDeathCandidate,
				T:          ctx.T,
				Confidence: ConfidenceLow,
				PreFight:   &evidence,
			})
		}
	}

	sort.SliceStable(out.Assessments, func(i, j int) bool { return out.Assessments[i].T < out.Assessments[j].T })
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].T == out.Candidates[j].T {
			return out.Candidates[i].Type < out.Candidates[j].Type
		}
		return out.Candidates[i].T < out.Candidates[j].T
	})
	return out
}

func assessIsolation(ctx timeline.TargetDeathContext) IsolationAssessment {
	evidence := IsolationEvidence{
		SupportRadiusWorld:    IsolationSupportRadiusWorld,
		SupportRadiusTimeline: IsolationSupportRadiusTimeline,
		DamageReceivedLast10s: ctx.DamageReceivedLast10s,
		DamageDealtLast10s:    ctx.DamageDealtLast10s,
		MissingEnemies:        []MissingEnemyEvidence{},
	}

	if ctx.Fight != nil {
		evidence.FightParticipants = len(ctx.Fight.Participants)
		evidence.FightDeaths = ctx.Fight.Deaths
		evidence.FightHeroDamage = ctx.Fight.HeroDamage
		evidence.TeamfightScale = evidence.FightParticipants >= TeamfightMinParticipants
	}

	for i, ally := range ctx.NearbyAllies {
		if i == 0 || evidence.NearestAllyDistance == nil || ally.Distance < *evidence.NearestAllyDistance {
			d := ally.Distance
			evidence.NearestAllyDistance = &d
		}
		if ally.Distance <= IsolationSupportRadiusTimeline {
			evidence.NearbyAlliesWithinSupport++
		}
	}

	for _, enemy := range ctx.EnemyKnowledge {
		if enemy.Status == "estimated_visible" {
			evidence.EstimatedVisibleEnemies++
			continue
		}
		missing := MissingEnemyEvidence{PlayerSlot: enemy.PlayerSlot, Status: enemy.Status}
		if enemy.SecondsSinceSeen != nil {
			age := *enemy.SecondsSinceSeen
			missing.SecondsSinceSeen = &age
		}
		evidence.MissingEnemies = append(evidence.MissingEnemies, missing)
	}

	// This is intentionally a candidate filter, not a mistake verdict. An
	// engagement below teamfight scale with no immediately supporting ally is
	// worth reviewing; sacrificial deaths and successful chases can still be
	// legitimate and must be handled by later context/calibration.
	candidate := !evidence.TeamfightScale && evidence.NearbyAlliesWithinSupport == 0
	return IsolationAssessment{Candidate: candidate, Evidence: evidence}
}

func assessPreFight(tl *timeline.MatchTimeline, target *timeline.PlayerTimeline, ctx timeline.TargetDeathContext) PreFightAssessment {
	evidence := PreFightEvidence{}
	if ctx.Fight != nil {
		evidence.CurrentFightTeamfightScale = len(ctx.Fight.Participants) >= TeamfightMinParticipants
	}

	deadConfirmed, respawnT := respawnAfterDeath(target, ctx.T)
	evidence.TargetDeathConfirmed = deadConfirmed
	evidence.RespawnT = respawnT
	if !deadConfirmed || evidence.CurrentFightTeamfightScale {
		return PreFightAssessment{Evidence: evidence}
	}

	next, ok := nextObservedTeamfight(tl.Fights, ctx.T)
	if !ok {
		return PreFightAssessment{Evidence: evidence}
	}
	evidence.ObservedFightTimingAvailable = true
	start := next.ObservedStartT
	evidence.NextTeamfightStartT = &start
	until := start - ctx.T
	evidence.SecondsUntilTeamfight = &until
	evidence.NextFightParticipants = len(next.Participants)
	evidence.NextFightDeaths = next.Deaths
	evidence.NextFightHeroDamage = next.HeroDamage

	if respawnT == nil || start < *respawnT {
		evidence.TargetDeadAtTeamfightStart = true
	}
	candidate := evidence.TargetDeadAtTeamfightStart
	return PreFightAssessment{Candidate: candidate, Evidence: evidence}
}

func nextObservedTeamfight(fights []timeline.FightWindow, afterT float64) (timeline.FightWindow, bool) {
	var best timeline.FightWindow
	found := false
	for _, fight := range fights {
		// Old timelines written before M7 have zero observed boundaries. Do not
		// silently substitute padded time; reprocessing the replay makes the
		// capability explicit and avoids ambiguous ordering.
		if fight.ObservedStartT <= 0 || fight.ObservedStartT <= afterT || len(fight.Participants) < TeamfightMinParticipants {
			continue
		}
		if !found || fight.ObservedStartT < best.ObservedStartT {
			best = fight
			found = true
		}
	}
	return best, found
}

func respawnAfterDeath(player *timeline.PlayerTimeline, deathT float64) (bool, *float64) {
	if player == nil {
		return false, nil
	}
	seenDead := false
	for _, sample := range player.Samples {
		if sample.T < deathT {
			continue
		}
		if !sample.Alive {
			seenDead = true
			continue
		}
		if seenDead {
			t := sample.T
			return true, &t
		}
	}
	return seenDead, nil
}

func targetPlayer(tl *timeline.MatchTimeline) *timeline.PlayerTimeline {
	if tl == nil {
		return nil
	}
	if player := tl.Players[strconv.Itoa(tl.TargetPlayerSlot)]; player != nil {
		return player
	}
	for _, player := range tl.Players {
		if player != nil && player.PlayerSlot == tl.TargetPlayerSlot {
			return player
		}
	}
	return nil
}
