package timeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJSON(storageRoot string, timeline MatchTimeline) (string, error) {
	dir := filepath.Join(storageRoot, "timelines")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create timeline directory: %w", err)
	}

	finalPath := filepath.Join(dir, fmt.Sprintf("%d.json", timeline.MatchID))
	data, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode timeline: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".timeline-*.json.tmp")
	if err != nil {
		return "", fmt.Errorf("create timeline temp file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("write timeline: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("sync timeline: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close timeline: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return "", fmt.Errorf("commit timeline: %w", err)
	}
	ok = true
	return finalPath, nil
}
