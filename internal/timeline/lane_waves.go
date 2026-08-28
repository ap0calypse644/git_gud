package timeline

import (
	"fmt"
	"math"
	"sort"
)

const (
	laneWaveMethod            = "observed_activation_cohort_spatial_tracks_diagonal_lane_evidence_v1"
	laneWaveTrackLinkWorld    = 900.0
	laneWaveTrackLinkTimeline = laneWaveTrackLinkWorld / 128.0
	laneWaveEvidenceWorld     = 1800.0
	laneWaveMinMidSamples     = 6
)

type rawWaveFrame struct {
	T          float64
	Components []rawWaveComponent
}

type rawWaveComponent struct {
	team            int
	cohortSecond    int
	centerX         float64
	centerY         float64
	creepCount      int
	laneCreepCount  int
	siegeCreepCount int
}

type rawWaveTrackSample struct {
	t               float64
	centerX         float64
	centerY         float64
	creepCount      int
	laneCreepCount  int
	siegeCreepCount int
}

type rawWaveTrack struct {
	team         int
	cohortSecond int
	lane         string
	samples      []rawWaveTrackSample
}

type waveCohortKey struct {
	team         int
	cohortSecond int
}

type waveOutputKey struct {
	team         int
	cohortSecond int
	lane         string
}

// cohortCreepComponents is the wave-identity counterpart of M12's general
// creep clustering. It only connects living creeps that share both team and
// observed activation cohort, preventing two consecutive waves that pile up in
// one lane from becoming the same wave merely because they are spatially close.
func cohortCreepComponents(states map[creepEntityKey]creepState, radius float64) []rawWaveComponent {
	points := make([]creepState, 0, len(states))
	for _, state := range states {
		if !state.alive || state.waiting || !state.cohortKnown {
			continue
		}
		points = append(points, state)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].team != points[j].team {
			return points[i].team < points[j].team
		}
		if points[i].cohortSecond != points[j].cohortSecond {
			return points[i].cohortSecond < points[j].cohortSecond
		}
		if points[i].key.index != points[j].key.index {
			return points[i].key.index < points[j].key.index
		}
		return points[i].key.serial < points[j].key.serial
	})
	if len(points) == 0 {
		return []rawWaveComponent{}
	}

	parent := make([]int, len(points))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	radius2 := radius * radius
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].team != points[j].team || points[i].cohortSecond != points[j].cohortSecond {
				continue
			}
			dx := points[i].x - points[j].x
			dy := points[i].y - points[j].y
			if dx*dx+dy*dy <= radius2 {
				union(i, j)
			}
		}
	}

	members := make(map[int][]int)
	for i := range points {
		root := find(i)
		members[root] = append(members[root], i)
	}

	components := make([]rawWaveComponent, 0, len(members))
	for _, indexes := range members {
		var sumX, sumY float64
		laneCount, siegeCount := 0, 0
		for _, idx := range indexes {
			point := points[idx]
			sumX += point.x
			sumY += point.y
			if point.kind == "siege" {
				siegeCount++
			} else {
				laneCount++
			}
		}
		components = append(components, rawWaveComponent{
			team:            points[indexes[0]].team,
			cohortSecond:    points[indexes[0]].cohortSecond,
			centerX:         sumX / float64(len(indexes)),
			centerY:         sumY / float64(len(indexes)),
			creepCount:      len(indexes),
			laneCreepCount:  laneCount,
			siegeCreepCount: siegeCount,
		})
	}

	sort.Slice(components, func(i, j int) bool {
		if components[i].team != components[j].team {
			return components[i].team < components[j].team
		}
		if components[i].cohortSecond != components[j].cohortSecond {
			return components[i].cohortSecond < components[j].cohortSecond
		}
		if components[i].centerX != components[j].centerX {
			return components[i].centerX < components[j].centerX
		}
		return components[i].centerY < components[j].centerY
	})
	return components
}

func deriveLaneWaveTimeline(frames []rawWaveFrame, evidence LaneWaveActivationEvidence, capability bool) LaneWaveTimeline {
	tracks := buildWaveTracks(frames)
	classifyWaveTracks(tracks)

	unknown := 0
	fragments := make(map[waveOutputKey]int)
	samplesByWave := make(map[waveOutputKey]map[int]LaneWaveSample)
	for _, track := range tracks {
		if track.lane == "" {
			unknown++
			continue
		}
		key := waveOutputKey{team: track.team, cohortSecond: track.cohortSecond, lane: track.lane}
		fragments[key]++
		bySecond := samplesByWave[key]
		if bySecond == nil {
			bySecond = make(map[int]LaneWaveSample)
			samplesByWave[key] = bySecond
		}
		for _, sample := range track.samples {
			second := int(math.Round(sample.t))
			current, exists := bySecond[second]
			if !exists {
				bySecond[second] = LaneWaveSample{
					T:               sample.t,
					CenterX:         sample.centerX,
					CenterY:         sample.centerY,
					CreepCount:      sample.creepCount,
					LaneCreepCount:  sample.laneCreepCount,
					SiegeCreepCount: sample.siegeCreepCount,
				}
				continue
			}

			// Spatial splits of one cohort/lane can create simultaneous track
			// fragments. Merge them with creep-count weighting rather than
			// selecting one fragment and losing replay-observed creeps.
			total := current.CreepCount + sample.creepCount
			if total > 0 {
				current.CenterX = (current.CenterX*float64(current.CreepCount) + sample.centerX*float64(sample.creepCount)) / float64(total)
				current.CenterY = (current.CenterY*float64(current.CreepCount) + sample.centerY*float64(sample.creepCount)) / float64(total)
			}
			current.CreepCount = total
			current.LaneCreepCount += sample.laneCreepCount
			current.SiegeCreepCount += sample.siegeCreepCount
			bySecond[second] = current
		}
	}

	waves := make([]LaneWave, 0, len(samplesByWave))
	for key, bySecond := range samplesByWave {
		seconds := make([]int, 0, len(bySecond))
		for second := range bySecond {
			seconds = append(seconds, second)
		}
		sort.Ints(seconds)
		if len(seconds) == 0 {
			continue
		}
		samples := make([]LaneWaveSample, 0, len(seconds))
		for _, second := range seconds {
			samples = append(samples, bySecond[second])
		}
		waves = append(waves, LaneWave{
			ID:             fmt.Sprintf("%d:%d:%s", key.team, key.cohortSecond, key.lane),
			Team:           key.team,
			SpawnT:         float64(key.cohortSecond),
			Lane:           key.lane,
			StartT:         samples[0].T,
			EndT:           samples[len(samples)-1].T,
			TrackFragments: fragments[key],
			Samples:        samples,
		})
	}

	sort.Slice(waves, func(i, j int) bool {
		if waves[i].SpawnT != waves[j].SpawnT {
			return waves[i].SpawnT < waves[j].SpawnT
		}
		if waves[i].Team != waves[j].Team {
			return waves[i].Team < waves[j].Team
		}
		return laneOrder(waves[i].Lane) < laneOrder(waves[j].Lane)
	})
	if waves == nil {
		waves = []LaneWave{}
	}

	return LaneWaveTimeline{
		Available:                  capability && len(waves) > 0,
		Method:                     laneWaveMethod,
		SampleIntervalSeconds:      1,
		SpawnCohortRoundingSeconds: 1,
		ComponentRadiusWorld:       creepClusterRadiusWorld,
		TrackLinkRadiusWorld:       laneWaveTrackLinkWorld,
		LaneEvidenceDistanceWorld:  laneWaveEvidenceWorld,
		UnknownTrackCount:          unknown,
		ActivationEvidence:         evidence,
		Waves:                      waves,
	}
}

func buildWaveTracks(frames []rawWaveFrame) []*rawWaveTrack {
	tracks := make([]*rawWaveTrack, 0)
	for _, frame := range frames {
		byCohort := make(map[waveCohortKey][]rawWaveComponent)
		for _, component := range frame.Components {
			key := waveCohortKey{team: component.team, cohortSecond: component.cohortSecond}
			byCohort[key] = append(byCohort[key], component)
		}

		keys := make([]waveCohortKey, 0, len(byCohort))
		for key := range byCohort {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].team != keys[j].team {
				return keys[i].team < keys[j].team
			}
			return keys[i].cohortSecond < keys[j].cohortSecond
		})

		for _, key := range keys {
			components := byCohort[key]
			activeTrackIndexes := make([]int, 0)
			for i, track := range tracks {
				if track.team != key.team || track.cohortSecond != key.cohortSecond || len(track.samples) == 0 {
					continue
				}
				last := track.samples[len(track.samples)-1]
				if last.t == frame.T-1 {
					activeTrackIndexes = append(activeTrackIndexes, i)
				}
			}

			type pair struct {
				trackIndex int
				compIndex  int
				distance   float64
			}
			pairs := make([]pair, 0)
			for _, trackIndex := range activeTrackIndexes {
				last := tracks[trackIndex].samples[len(tracks[trackIndex].samples)-1]
				for compIndex, component := range components {
					d := math.Hypot(last.centerX-component.centerX, last.centerY-component.centerY)
					if d <= laneWaveTrackLinkTimeline {
						pairs = append(pairs, pair{trackIndex: trackIndex, compIndex: compIndex, distance: d})
					}
				}
			}
			sort.Slice(pairs, func(i, j int) bool {
				if pairs[i].distance != pairs[j].distance {
					return pairs[i].distance < pairs[j].distance
				}
				if pairs[i].trackIndex != pairs[j].trackIndex {
					return pairs[i].trackIndex < pairs[j].trackIndex
				}
				return pairs[i].compIndex < pairs[j].compIndex
			})

			usedTracks := make(map[int]bool)
			usedComponents := make(map[int]bool)
			for _, candidate := range pairs {
				if usedTracks[candidate.trackIndex] || usedComponents[candidate.compIndex] {
					continue
				}
				component := components[candidate.compIndex]
				tracks[candidate.trackIndex].samples = append(tracks[candidate.trackIndex].samples, rawWaveTrackSample{
					t:               frame.T,
					centerX:         component.centerX,
					centerY:         component.centerY,
					creepCount:      component.creepCount,
					laneCreepCount:  component.laneCreepCount,
					siegeCreepCount: component.siegeCreepCount,
				})
				usedTracks[candidate.trackIndex] = true
				usedComponents[candidate.compIndex] = true
			}

			for compIndex, component := range components {
				if usedComponents[compIndex] {
					continue
				}
				tracks = append(tracks, &rawWaveTrack{
					team:         key.team,
					cohortSecond: key.cohortSecond,
					samples: []rawWaveTrackSample{{
						t:               frame.T,
						centerX:         component.centerX,
						centerY:         component.centerY,
						creepCount:      component.creepCount,
						laneCreepCount:  component.laneCreepCount,
						siegeCreepCount: component.siegeCreepCount,
					}},
				})
			}
		}
	}
	return tracks
}

// classifyWaveTracks uses only map-invariant lane structure in Source 2's
// coordinate system: top lane moves to the y>x side of the main diagonal,
// bottom to x>y, while mid remains near the diagonal. Side-lane labels require
// strong off-diagonal evidence. Mid is inferred conservatively as the longest
// remaining track only when the same team/cohort also exposes both side lanes.
// Short ambiguous base fragments therefore remain unknown.
func classifyWaveTracks(tracks []*rawWaveTrack) {
	byCohort := make(map[waveCohortKey][]int)
	for i, track := range tracks {
		key := waveCohortKey{team: track.team, cohortSecond: track.cohortSecond}
		byCohort[key] = append(byCohort[key], i)
	}

	for _, indexes := range byCohort {
		hasTop, hasBottom := false, false
		for _, idx := range indexes {
			positive, negative := false, false
			for _, sample := range tracks[idx].samples {
				deltaWorld := (sample.centerY - sample.centerX) * 128.0
				if deltaWorld >= laneWaveEvidenceWorld {
					positive = true
				}
				if deltaWorld <= -laneWaveEvidenceWorld {
					negative = true
				}
			}
			switch {
			case positive && !negative:
				tracks[idx].lane = "top"
				hasTop = true
			case negative && !positive:
				tracks[idx].lane = "bottom"
				hasBottom = true
			}
		}

		if !hasTop || !hasBottom {
			continue
		}

		midIndex := -1
		for _, idx := range indexes {
			if tracks[idx].lane != "" || len(tracks[idx].samples) < laneWaveMinMidSamples {
				continue
			}
			if midIndex < 0 || betterMidTrack(tracks[idx], tracks[midIndex]) {
				midIndex = idx
			}
		}
		if midIndex >= 0 {
			tracks[midIndex].lane = "mid"
		}
	}
}

func betterMidTrack(a, b *rawWaveTrack) bool {
	if len(a.samples) != len(b.samples) {
		return len(a.samples) > len(b.samples)
	}
	meanA := meanAbsDiagonalDistanceWorld(a)
	meanB := meanAbsDiagonalDistanceWorld(b)
	if meanA != meanB {
		return meanA < meanB
	}
	if len(a.samples) == 0 || len(b.samples) == 0 {
		return false
	}
	return a.samples[0].t < b.samples[0].t
}

func meanAbsDiagonalDistanceWorld(track *rawWaveTrack) float64 {
	if len(track.samples) == 0 {
		return math.Inf(1)
	}
	var total float64
	for _, sample := range track.samples {
		total += math.Abs(sample.centerY-sample.centerX) * 128.0
	}
	return total / float64(len(track.samples))
}

func laneOrder(lane string) int {
	switch lane {
	case "top":
		return 0
	case "mid":
		return 1
	case "bottom":
		return 2
	default:
		return 3
	}
}
