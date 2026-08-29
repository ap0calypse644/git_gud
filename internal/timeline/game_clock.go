package timeline

import "github.com/dotabuff/manta"

// replayGameClock maps server net ticks onto Dota's pause-aware game clock.
// Combat-log timestamps already use this clock; entity-derived timestamps must
// subtract accumulated paused ticks to stay aligned with them.
type replayGameClock struct {
	paused           bool
	pauseStartTick   int
	totalPausedTicks int
}

func newReplayGameClock() *replayGameClock {
	return &replayGameClock{}
}

func (c *replayGameClock) observe(e *manta.Entity) {
	if e.GetClassName() != "CDOTAGamerulesProxy" {
		return
	}
	if paused, ok := e.GetBool("m_pGameRules.m_bGamePaused"); ok {
		c.paused = paused
	}
	if tick, ok := numberInt(e.Get("m_pGameRules.m_nPauseStartTick")); ok {
		c.pauseStartTick = tick
	}
	if ticks, ok := numberInt(e.Get("m_pGameRules.m_nTotalPausedTicks")); ok && ticks >= 0 {
		c.totalPausedTicks = ticks
	}
}

func (c *replayGameClock) absoluteTime(netTick uint32) float64 {
	return pauseAwareAbsoluteTime(netTick, c.paused, c.pauseStartTick, c.totalPausedTicks)
}

func pauseAwareAbsoluteTime(netTick uint32, paused bool, pauseStartTick, totalPausedTicks int) float64 {
	timeTick := int64(netTick)
	if paused && pauseStartTick > 0 && int64(pauseStartTick) < timeTick {
		timeTick = int64(pauseStartTick)
	}
	pausedTicks := int64(totalPausedTicks)
	if pausedTicks < 0 {
		pausedTicks = 0
	}
	if pausedTicks > timeTick {
		pausedTicks = timeTick
	}
	return float64(timeTick-pausedTicks) / tickRate
}
