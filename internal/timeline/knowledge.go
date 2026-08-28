package timeline

import (
	"math"
	"sort"
)

const (
	worldUnitsPerTimelineCoord = 128.0
	estimatedVisibilityMaxGap  = 2.5
	heroSourceTimeTolerance    = 1.25
)

// DeriveKnowledge builds conservative, explicitly estimated team-information
// inputs. Exact replay hero positions are used only to test whether an enemy
// sample falls inside a replay-observed friendly source's nominal vision
// radius. Terrain, trees, temporary darkness, invisibility and other FoW
// mechanics are not reconstructed here, so these intervals must never be
// treated as direct visibility proof.
func DeriveKnowledge(tl *MatchTimeline) KnowledgeTimeline {
	out := KnowledgeTimeline{
		Method: "observer_ward_and_allied_hero_conservative_radius",
	}

	target := tl.Players[slotKey(tl.TargetPlayerSlot)]
	if target == nil {
		out.EstimatedVisibility = []EstimatedVisibilityInterval{}
		return out
	}
	out.Team = target.Team
	heroIndex := indexHeroVisionSources(target.Team, tl.VisionSources.Heroes)

	for _, player := range tl.Players {
		if player == nil || player.Team == target.Team {
			continue
		}
		out.EstimatedVisibility = append(out.EstimatedVisibility,
			deriveEstimatedVisibilityForPlayer(player, target.Team, tl.VisionSources.Wards, heroIndex)...)
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

func deriveEstimatedVisibilityForPlayer(player *PlayerTimeline, viewerTeam int, wards []WardInterval, heroIndex map[int][]HeroVisionSample) []EstimatedVisibilityInterval {
	var out []EstimatedVisibilityInterval
	var current *EstimatedVisibilityInterval

	flush := func() {
		if current == nil {
			return
		}
		current.SourceWards = uniqueWardRefs(current.SourceWards)
		current.SourceHeroSlots = uniqueInts(current.SourceHeroSlots)
		out = append(out, *current)
		current = nil
	}

	for _, sample := range player.Samples {
		if !sample.Alive {
			flush()
			continue
		}

		wardSources := observerWardsCoveringSample(sample, viewerTeam, wards)
		heroSources := alliedHeroesCoveringSample(sample, viewerTeam, heroIndex)
		if len(wardSources) == 0 && len(heroSources) == 0 {
			flush()
			continue
		}

		if current == nil || sample.T-current.EndT > estimatedVisibilityMaxGap {
			flush()
			current = &EstimatedVisibilityInterval{
				PlayerSlot:      player.PlayerSlot,
				StartT:          sample.T,
				EndT:            sample.T,
				StartX:          sample.X,
				StartY:          sample.Y,
				EndX:            sample.X,
				EndY:            sample.Y,
				SampleCount:     1,
				SourceWards:     append([]VisionSourceRef(nil), wardSources...),
				SourceHeroSlots: append([]int(nil), heroSources...),
			}
			continue
		}

		current.EndT = sample.T
		current.EndX = sample.X
		current.EndY = sample.Y
		current.SampleCount++
		current.SourceWards = append(current.SourceWards, wardSources...)
		current.SourceHeroSlots = append(current.SourceHeroSlots, heroSources...)
	}
	flush()
	return out
}

// Retained as a focused helper for tests and auditing of the previously
// validated ward-only behavior.
func deriveEstimatedWardVisibilityForPlayer(player *PlayerTimeline, viewerTeam int, wards []WardInterval) []EstimatedVisibilityInterval {
	return deriveEstimatedVisibilityForPlayer(player, viewerTeam, wards, nil)
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
		rangeWorld := conservativeVisionRange(ward.DayVisionRange, ward.NightVisionRange)
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

func indexHeroVisionSources(viewerTeam int, sources []HeroVisionSample) map[int][]HeroVisionSample {
	out := make(map[int][]HeroVisionSample)
	for _, source := range sources {
		if source.Team != viewerTeam || !source.Alive {
			continue
		}
		if conservativeVisionRange(source.DayVisionRange, source.NightVisionRange) <= 0 {
			continue
		}
		sec := int(math.Floor(source.T))
		out[sec] = append(out[sec], source)
	}
	return out
}

func alliedHeroesCoveringSample(sample HeroSample, viewerTeam int, index map[int][]HeroVisionSample) []int {
	if len(index) == 0 {
		return nil
	}
	sec := int(math.Floor(sample.T))
	var slots []int
	for bucket := sec - 1; bucket <= sec+1; bucket++ {
		for _, source := range index[bucket] {
			if source.Team != viewerTeam || !source.Alive || math.Abs(source.T-sample.T) > heroSourceTimeTolerance {
				continue
			}
			rangeWorld := conservativeVisionRange(source.DayVisionRange, source.NightVisionRange)
			if rangeWorld <= 0 {
				continue
			}
			radius := rangeWorld / worldUnitsPerTimelineCoord
			dx := sample.X - source.X
			dy := sample.Y - source.Y
			if dx*dx+dy*dy <= radius*radius {
				slots = append(slots, source.PlayerSlot)
			}
		}
	}
	return uniqueInts(slots)
}

// If replay-provided day/night ranges differ, use the smaller positive radius.
// This deliberately underclaims ordinary daytime vision rather than pretending
// we have reconstructed temporary darkness and every patch-specific modifier.
func conservativeVisionRange(day, night float64) float64 {
	switch {
	case day > 0 && night > 0 && day < night:
		return day
	case day > 0 && night > 0:
		return night
	case day > 0:
		return day
	default:
		return night
	}
}

func conservativeWardVisionRange(ward WardInterval) float64 {
	return conservativeVisionRange(ward.DayVisionRange, ward.NightVisionRange)
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

func uniqueInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
