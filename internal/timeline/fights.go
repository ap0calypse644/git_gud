package timeline

import (
	"math"
	"sort"
)

const (
	fightGapSeconds        = 5.0
	fightLeadSeconds       = 3.0
	fightTrailSeconds      = 5.0
	fightMaxRawSpanSeconds = 45.0
	fightSpatialRadius     = 12.0
	fightSampleMaxAge      = 4.0
	fightMinDamage         = int64(700)
	fightMinTwoHeroDamage  = int64(1400)
	fightMinDamagePerSec   = 70.0
)

type fightMoment struct {
	t       float64
	slots   []int
	damage  int64
	isDeath bool
	x       float64
	y       float64
	hasPos  bool
}

type fightAggregate struct {
	first        float64
	last         float64
	participants map[int]struct{}
	deaths       int
	damage       int64
	posXSum      float64
	posYSum      float64
	posCount     int
	lastX        float64
	lastY        float64
	hasLastPos   bool
}

// DeriveFightWindows clusters hero-to-hero damage and hero deaths into
// deterministic combat windows. Time alone is not enough: during laning there
// is almost always hero damage somewhere on the map, so a global 10-second gap
// chains unrelated top/mid/bot trades into multi-minute "fights". We therefore
// keep multiple spatially distinct combat clusters alive at once and only join
// moments that are both temporally and geographically close.
func DeriveFightWindows(timeline *MatchTimeline) []FightWindow {
	if timeline == nil {
		return nil
	}

	moments := make([]fightMoment, 0, len(timeline.Damage)+len(timeline.Deaths))
	for _, d := range timeline.Damage {
		if d.Value <= 0 || d.AttackerSlot == d.VictimSlot {
			continue
		}
		x, y, ok := meanPositionAt(timeline, d.T, []int{d.AttackerSlot, d.VictimSlot})
		moments = append(moments, fightMoment{
			t:      d.T,
			slots:  []int{d.AttackerSlot, d.VictimSlot},
			damage: int64(d.Value),
			x:      x,
			y:      y,
			hasPos: ok,
		})
	}
	for _, d := range timeline.Deaths {
		slots := make([]int, 0, 2+len(d.AssistSlots))
		if d.AttackerSlot != nil {
			slots = append(slots, *d.AttackerSlot)
		}
		if d.VictimSlot != nil {
			slots = append(slots, *d.VictimSlot)
		}
		slots = append(slots, d.AssistSlots...)
		if len(slots) == 0 {
			continue
		}

		var x, y float64
		var ok bool
		if d.VictimSlot != nil {
			x, y, ok = playerPositionAt(timeline, *d.VictimSlot, d.T)
		}
		if !ok {
			x, y, ok = meanPositionAt(timeline, d.T, slots)
		}
		moments = append(moments, fightMoment{
			t: d.T, slots: slots, isDeath: true, x: x, y: y, hasPos: ok,
		})
	}
	if len(moments) == 0 {
		return nil
	}

	sort.Slice(moments, func(i, j int) bool { return moments[i].t < moments[j].t })

	clusters := make([]fightAggregate, 0, 32)
	for _, m := range moments {
		best := -1
		bestDistance := math.MaxFloat64

		for i := range clusters {
			a := &clusters[i]
			if m.t-a.last > fightGapSeconds || m.t-a.first > fightMaxRawSpanSeconds {
				continue
			}

			shared := sharesParticipant(a.participants, m.slots)
			if m.hasPos && a.hasLastPos {
				d := math.Hypot(m.x-a.lastX, m.y-a.lastY)
				if d > fightSpatialRadius {
					continue
				}
				// Prefer the geographically nearest active cluster. A slight
				// shared-participant bias helps moving/chasing fights remain intact.
				if shared {
					d *= 0.75
				}
				if d < bestDistance {
					best = i
					bestDistance = d
				}
				continue
			}

			// If either side lacks a usable position, only join on a very recent
			// shared participant. This is deliberately conservative: missing
			// coordinates must never become a license to merge combat map-wide.
			if shared && m.t-a.last <= 2.0 && best < 0 {
				best = i
			}
		}

		if best < 0 {
			clusters = append(clusters, newFightAggregate(m))
			continue
		}
		addFightMoment(&clusters[best], m)
	}

	windows := make([]FightWindow, 0, len(clusters))
	for _, a := range clusters {
		if w, ok := finalizeFight(timeline, a); ok {
			windows = append(windows, w)
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].StartT < windows[j].StartT })
	return windows
}

func newFightAggregate(m fightMoment) fightAggregate {
	a := fightAggregate{first: m.t, last: m.t, participants: make(map[int]struct{})}
	addFightMoment(&a, m)
	return a
}

func addFightMoment(a *fightAggregate, m fightMoment) {
	if a.participants == nil {
		a.participants = make(map[int]struct{})
	}
	if a.first == 0 && a.last == 0 && m.t != 0 {
		a.first = m.t
	}
	if m.t < a.first {
		a.first = m.t
	}
	if m.t > a.last {
		a.last = m.t
	}
	for _, slot := range m.slots {
		a.participants[slot] = struct{}{}
	}
	a.damage += m.damage
	if m.isDeath {
		a.deaths++
	}
	if m.hasPos {
		a.posXSum += m.x
		a.posYSum += m.y
		a.posCount++
		a.lastX = m.x
		a.lastY = m.y
		a.hasLastPos = true
	}
}

func finalizeFight(timeline *MatchTimeline, a fightAggregate) (FightWindow, bool) {
	if len(a.participants) < 2 {
		return FightWindow{}, false
	}

	rawSpan := a.last - a.first
	if rawSpan < 1 {
		rawSpan = 1
	}
	if a.deaths == 0 {
		if a.damage < fightMinDamage {
			return FightWindow{}, false
		}
		if len(a.participants) == 2 && a.damage < fightMinTwoHeroDamage {
			return FightWindow{}, false
		}
		if float64(a.damage)/rawSpan < fightMinDamagePerSec {
			return FightWindow{}, false
		}
	}

	participants := make([]int, 0, len(a.participants))
	targetInvolved := false
	for slot := range a.participants {
		participants = append(participants, slot)
		if slot == timeline.TargetPlayerSlot {
			targetInvolved = true
		}
	}
	sort.Ints(participants)

	start := a.first - fightLeadSeconds
	if start < 0 {
		start = 0
	}
	end := a.last + fightTrailSeconds
	if timeline.DurationSeconds > 0 && end > timeline.DurationSeconds {
		end = timeline.DurationSeconds
	}

	w := FightWindow{
		StartT:         start,
		EndT:           end,
		Participants:   participants,
		Deaths:         a.deaths,
		HeroDamage:     a.damage,
		TargetInvolved: targetInvolved,
	}
	if a.posCount > 0 {
		w.CenterX = a.posXSum / float64(a.posCount)
		w.CenterY = a.posYSum / float64(a.posCount)
	}
	return w, true
}

func sharesParticipant(existing map[int]struct{}, slots []int) bool {
	for _, slot := range slots {
		if _, ok := existing[slot]; ok {
			return true
		}
	}
	return false
}

func meanPositionAt(timeline *MatchTimeline, t float64, slots []int) (float64, float64, bool) {
	var xSum, ySum float64
	count := 0
	seen := make(map[int]struct{}, len(slots))
	for _, slot := range slots {
		if _, ok := seen[slot]; ok {
			continue
		}
		seen[slot] = struct{}{}
		x, y, ok := playerPositionAt(timeline, slot, t)
		if !ok {
			continue
		}
		xSum += x
		ySum += y
		count++
	}
	if count == 0 {
		return 0, 0, false
	}
	return xSum / float64(count), ySum / float64(count), true
}

func playerPositionAt(timeline *MatchTimeline, slot int, t float64) (float64, float64, bool) {
	if timeline == nil {
		return 0, 0, false
	}
	player := timeline.Players[slotKey(slot)]
	if player == nil || len(player.Samples) == 0 {
		return 0, 0, false
	}

	samples := player.Samples
	i := sort.Search(len(samples), func(i int) bool { return samples[i].T >= t })
	best := -1
	bestDelta := math.MaxFloat64
	if i < len(samples) {
		best = i
		bestDelta = math.Abs(samples[i].T - t)
	}
	if i > 0 {
		d := math.Abs(samples[i-1].T - t)
		if d < bestDelta {
			best = i - 1
			bestDelta = d
		}
	}
	if best < 0 || bestDelta > fightSampleMaxAge {
		return 0, 0, false
	}
	return samples[best].X, samples[best].Y, true
}
