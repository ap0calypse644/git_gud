package timeline

import (
	"math"
	"sort"
)

const (
	worldUnitsPerTimelineCoord = 128.0
	estimatedVisibilityMaxGap  = 2.5
)

// DeriveKnowledge builds conservative, explicitly estimated team-information
// inputs. Exact replay hero positions are used only to test whether a sample
// falls inside a replay-observed observer ward's nominal radius. Terrain,
// trees, temporary day/night effects, invisibility and other FoW mechanics are
// not reconstructed here, so these intervals must never be treated as direct
// visibility proof.
func DeriveKnowledge(tl *MatchTimeline) KnowledgeTimeline {
	out := KnowledgeTimeline{
		Method: "observer_ward_radius_only",
	}

	target := tl.Players[slotKey(tl.TargetPlayerSlot)]
	if target == nil {
		out.EstimatedVisibility = []EstimatedVisibilityInterval{}
		return out
	}
	out.Team = target.Team

	for _, player := range tl.Players {
		if player == nil || player.Team == target.Team {
			continue
		}
		out.EstimatedVisibility = append(out.EstimatedVisibility,
			deriveEstimatedWardVisibilityForPlayer(player, target.Team, tl.VisionSources.Wards)...)
	}

	sort.Slice(out.EstimatedVisibility, func(i, j int) bool {
		if out.EstimatedVisibility[i].StartT == out.EstimatedVisibility[j].StartT {
			return out.EstimatedVisibility[i].PlayerSlot < out.EstimatedVisibility[j].PlayerSlot
		}
		return out.EstimatedVisibility[i].StartT < out.EstimatedVisibility[j].StartT
	})
	if out.EstimatedVisibility == nil {
		out.EstimatedVisibility = []EstimatedVisibilityInterval{}
	}
	return out
}

func deriveEstimatedWardVisibilityForPlayer(player *PlayerTimeline, viewerTeam int, wards []WardInterval) []EstimatedVisibilityInterval {
	var out []EstimatedVisibilityInterval
	var current *EstimatedVisibilityInterval

	flush := func() {
		if current == nil {
			return
		}
		current.SourceWards = uniqueWardRefs(current.SourceWards)
		out = append(out, *current)
		current = nil
	}

	for _, sample := range player.Samples {
		if !sample.Alive {
			flush()
			continue
		}

		sources := observerWardsCoveringSample(sample, viewerTeam, wards)
		if len(sources) == 0 {
			flush()
			continue
		}

		if current == nil || sample.T-current.EndT > estimatedVisibilityMaxGap {
			flush()
			current = &EstimatedVisibilityInterval{
				PlayerSlot:  player.PlayerSlot,
				StartT:      sample.T,
				EndT:        sample.T,
				StartX:      sample.X,
				StartY:      sample.Y,
				EndX:        sample.X,
				EndY:        sample.Y,
				SampleCount: 1,
				SourceWards: append([]VisionSourceRef(nil), sources...),
			}
			continue
		}

		current.EndT = sample.T
		current.EndX = sample.X
		current.EndY = sample.Y
		current.SampleCount++
		current.SourceWards = append(current.SourceWards, sources...)
	}
	flush()
	return out
}

func observerWardsCoveringSample(sample HeroSample, viewerTeam int, wards []WardInterval) []VisionSourceRef {
	var refs []VisionSourceRef
	for _, ward := range wards {
		if ward.Kind != "observer" || ward.Team != viewerTeam {
			continue
		}
		if sample.T < ward.StartT || sample.T > ward.EndT {
			continue
		}
		rangeWorld := wardVisionRangeAt(ward, sample.T)
		if rangeWorld <= 0 {
			continue
		}
		radius := rangeWorld / worldUnitsPerTimelineCoord
		dx := sample.X - ward.X
		dy := sample.Y - ward.Y
		if dx*dx+dy*dy > radius*radius {
			continue
		}
		refs = append(refs, VisionSourceRef{EntityIndex: ward.EntityIndex, EntitySerial: ward.EntitySerial})
	}
	return refs
}

// wardVisionRangeAt uses the ordinary five-minute day/night cadence only when
// the replay reports different day/night ranges. Temporary darkness and
// hero-specific vision modifiers are intentionally outside this estimate.
func wardVisionRangeAt(ward WardInterval, t float64) float64 {
	if ward.DayVisionRange == ward.NightVisionRange {
		return ward.DayVisionRange
	}
	if ward.DayVisionRange <= 0 {
		return ward.NightVisionRange
	}
	if ward.NightVisionRange <= 0 {
		return ward.DayVisionRange
	}
	phase := int(math.Floor(math.Max(t, 0)/300.0)) % 2
	if phase == 0 {
		return ward.DayVisionRange
	}
	return ward.NightVisionRange
}

func uniqueWardRefs(in []VisionSourceRef) []VisionSourceRef {
	seen := make(map[VisionSourceRef]struct{}, len(in))
	out := make([]VisionSourceRef, 0, len(in))
	for _, ref := range in {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntityIndex == out[j].EntityIndex {
			return out[i].EntitySerial < out[j].EntitySerial
		}
		return out[i].EntityIndex < out[j].EntityIndex
	})
	return out
}
