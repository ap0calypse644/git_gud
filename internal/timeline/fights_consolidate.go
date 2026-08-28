package timeline

import (
	"math"
	"sort"
)

const (
	fightMergeMinOverlapSeconds = 2.5
	fightMergeSpatialRadius     = 15.0
	fightMergeParticipantRatio  = 0.60
)

// ConsolidateFightWindows collapses duplicate views of the same engagement
// after the spatial clusterer has finished. Separate clusters can legitimately
// form while a fight is split into nearby pockets and then converge later. If
// their padded windows overlap substantially, their centers are close, and
// most of the smaller participant set is shared, they are one engagement for
// coaching purposes rather than two simultaneous fights.
//
// The input windows are derived from disjoint combat moments, so summing deaths
// and damage when two windows merge does not double-count raw replay events.
func ConsolidateFightWindows(windows []FightWindow) []FightWindow {
	if len(windows) < 2 {
		return windows
	}

	current := append([]FightWindow(nil), windows...)
	sort.Slice(current, func(i, j int) bool { return current[i].StartT < current[j].StartT })

	// A merge can make a window eligible to merge with an earlier neighbor.
	// Re-run until a full pass makes no changes. Each changed pass reduces the
	// number of windows, so this always terminates.
	for {
		next := make([]FightWindow, 0, len(current))
		changed := false

		for _, candidate := range current {
			merged := false
			for i := len(next) - 1; i >= 0; i-- {
				if next[i].EndT < candidate.StartT {
					break
				}
				if !shouldMergeFightWindows(next[i], candidate) {
					continue
				}
				next[i] = mergeFightWindows(next[i], candidate)
				merged = true
				changed = true
				break
			}
			if !merged {
				next = append(next, candidate)
			}
		}

		sort.Slice(next, func(i, j int) bool { return next[i].StartT < next[j].StartT })
		if !changed {
			return next
		}
		current = next
	}
}

func shouldMergeFightWindows(a, b FightWindow) bool {
	overlap := math.Min(a.EndT, b.EndT) - math.Max(a.StartT, b.StartT)
	if overlap < fightMergeMinOverlapSeconds {
		return false
	}

	// A zero center means the source fight had no usable replay positions.
	// Missing coordinates must not become a reason to merge combat globally.
	if (a.CenterX == 0 && a.CenterY == 0) || (b.CenterX == 0 && b.CenterY == 0) {
		return false
	}
	if math.Hypot(a.CenterX-b.CenterX, a.CenterY-b.CenterY) > fightMergeSpatialRadius {
		return false
	}

	intersection := participantIntersectionCount(a.Participants, b.Participants)
	if intersection < 2 {
		return false
	}
	denom := len(a.Participants)
	if len(b.Participants) < denom {
		denom = len(b.Participants)
	}
	if denom == 0 {
		return false
	}
	return float64(intersection)/float64(denom) >= fightMergeParticipantRatio
}

func mergeFightWindows(a, b FightWindow) FightWindow {
	out := FightWindow{
		StartT:         math.Min(a.StartT, b.StartT),
		EndT:           math.Max(a.EndT, b.EndT),
		Participants:   unionParticipants(a.Participants, b.Participants),
		Deaths:         a.Deaths + b.Deaths,
		HeroDamage:     a.HeroDamage + b.HeroDamage,
		TargetInvolved: a.TargetInvolved || b.TargetInvolved,
	}

	// Damage is a useful proxy for how much of the engagement happened around
	// each cluster center. Death-only windows still receive a small non-zero
	// weight so their location is not discarded.
	wa := float64(a.HeroDamage)
	wb := float64(b.HeroDamage)
	if wa <= 0 {
		wa = 1
	}
	if wb <= 0 {
		wb = 1
	}
	out.CenterX = (a.CenterX*wa + b.CenterX*wb) / (wa + wb)
	out.CenterY = (a.CenterY*wa + b.CenterY*wb) / (wa + wb)
	return out
}

func participantIntersectionCount(a, b []int) int {
	seen := make(map[int]struct{}, len(a))
	for _, slot := range a {
		seen[slot] = struct{}{}
	}
	count := 0
	for _, slot := range b {
		if _, ok := seen[slot]; ok {
			count++
		}
	}
	return count
}

func unionParticipants(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	for _, slot := range a {
		seen[slot] = struct{}{}
	}
	for _, slot := range b {
		seen[slot] = struct{}{}
	}
	out := make([]int, 0, len(seen))
	for slot := range seen {
		out = append(out, slot)
	}
	sort.Ints(out)
	return out
}
