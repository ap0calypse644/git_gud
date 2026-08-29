package timeline

import (
	"context"
	"fmt"
	"os"

	"github.com/ap0calypse644/git_gud/internal/opendota"
)

type Builder struct {
	storageRoot string
	accountID   uint32
}

func NewBuilder(storageRoot string, accountID uint32) *Builder {
	return &Builder{storageRoot: storageRoot, accountID: accountID}
}

// Build parses one acquired replay and writes its deterministic timeline JSON.
// context is accepted as part of the shared processing interface; Manta itself
// is synchronous and currently cannot be interrupted mid-parse.
func (b *Builder) Build(_ context.Context, match opendota.Match, replayPath string) (string, error) {
	f, err := os.Open(replayPath)
	if err != nil {
		return "", fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()

	targetSlot := -1
	if player, ok := match.PlayerByAccountID(b.accountID); ok {
		targetSlot = player.PlayerSlot
	}

	parsed, err := Parse(f, ParseOptions{
		MatchID:          match.MatchID,
		AccountID:        b.accountID,
		TargetPlayerSlot: targetSlot,
	})
	if err != nil {
		return "", err
	}

	parsed.Fights = ConsolidateFightWindows(parsed.Fights)
	if parsed.Visibility.Events == nil {
		parsed.Visibility.Events = []VisibilityEvent{}
	}
	parsed.Knowledge = DeriveKnowledge(&parsed)
	parsed.LaneStructures = DeriveLaneStructures(&parsed)
	parsed.TargetWaveTaking = DeriveTargetWaveTaking(&parsed)
	parsed.TargetWaveDanger = DeriveTargetWaveDangerContext(&parsed, parsed.LaneTowerPositions)
	parsed.TargetFightContexts = DeriveTargetFightContexts(&parsed)
	parsed.TargetPostFightObjectives = DerivePostFightObjectiveTimeline(&parsed)
	parsed.TargetPostWaveOverstay = DeriveTargetPostWaveOverstay(&parsed)
	parsed.TargetDeathContexts = DeriveTargetDeathContexts(&parsed)
	return WriteJSON(b.storageRoot, parsed)
}
