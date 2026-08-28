package timeline

import "sort"

const (
	fightGapSeconds   = 10.0
	fightLeadSeconds  = 3.0
	fightTrailSeconds = 5.0
)

type fightMoment struct {
	t       float64
	slots   []int
	damage  int64
	isDeath bool
}

// DeriveFightWindows clusters hero-to-hero damage and hero deaths into
// deterministic combat windows. The output is intentionally descriptive: it
// says when sustained combat happened and who was involved, not whether the
// player should have joined it.
func DeriveFightWindows(timeline *MatchTimeline) []FightWindow {
	if timeline == nil {
		return nil
	}

	moments := make([]fightMoment, 0, len(timeline.Damage)+len(timeline.Deaths))
	for _, d := range timeline.Damage {
		if d.Value <= 0 || d.AttackerSlot == d.VictimSlot {
			continue
		}
		moments = append(moments, fightMoment{
			t:      d.T,
			slots:  []int{d.AttackerSlot, d.VictimSlot},
			damage: int64(d.Value),
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
		moments = append(moments, fightMoment{t: d.T, slots: slots, isDeath: true})
	}
	if len(moments) == 0 {
		return nil
	}

	sort.Slice(moments, func(i, j int) bool { return moments[i].t < moments[j].t })

	type aggregate struct {
		first        float64
		last         float64
		participants map[int]struct{}
		deaths       int
		damage       int64
	}

	flush := func(a aggregate, dst []FightWindow) []FightWindow {
		if len(a.participants) < 2 || (a.deaths == 0 && a.damage < 500) {
			return dst
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
		return append(dst, FightWindow{
			StartT:         start,
			EndT:           end,
			Participants:   participants,
			Deaths:         a.deaths,
			HeroDamage:     a.damage,
			TargetInvolved: targetInvolved,
		})
	}

	windows := make([]FightWindow, 0)
	current := aggregate{participants: make(map[int]struct{})}
	for i, m := range moments {
		if i == 0 {
			current.first = m.t
			current.last = m.t
		} else if m.t-current.last > fightGapSeconds {
			windows = flush(current, windows)
			current = aggregate{first: m.t, last: m.t, participants: make(map[int]struct{})}
		}
		current.last = m.t
		for _, slot := range m.slots {
			current.participants[slot] = struct{}{}
		}
		current.damage += m.damage
		if m.isDeath {
			current.deaths++
		}
	}
	return flush(current, windows)
}
