package timeline

import (
	"math"
	"testing"
)

func TestPauseAwareAbsoluteTimeSubtractsCompletedPauses(t *testing.T) {
	got := pauseAwareAbsoluteTime(85568, false, 0, 1100)
	want := float64(85568-1100) / tickRate
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("absolute time = %v, want %v", got, want)
	}
}

func TestPauseAwareAbsoluteTimeFreezesDuringPause(t *testing.T) {
	got := pauseAwareAbsoluteTime(24500, true, 24088, 105)
	want := float64(24088-105) / tickRate
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("paused absolute time = %v, want %v", got, want)
	}
}

func TestPauseAwareAbsoluteTimeFallsBackWithoutPauseMetadata(t *testing.T) {
	got := pauseAwareAbsoluteTime(9000, false, 0, 0)
	want := 300.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("absolute time = %v, want %v", got, want)
	}
}
