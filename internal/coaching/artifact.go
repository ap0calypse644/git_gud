package coaching

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReportArtifactWriter keeps report persistence on the compact side of the AI
// boundary. It receives only MatchCoachingInput, asks a ReportGenerator for a
// structured report, and atomically stores that report under the configured
// storage directory.
type ReportArtifactWriter struct {
	storagePath string
	generator   ReportGenerator
}

func NewReportArtifactWriter(storagePath string, generator ReportGenerator) *ReportArtifactWriter {
	return &ReportArtifactWriter{storagePath: storagePath, generator: generator}
}

func (w *ReportArtifactWriter) Generate(ctx context.Context, input MatchCoachingInput) (string, error) {
	if w == nil {
		return "", fmt.Errorf("coaching report artifact writer is nil")
	}
	if w.generator == nil {
		return "", fmt.Errorf("coaching report generator is nil")
	}
	if input.MatchID <= 0 {
		return "", fmt.Errorf("coaching input match_id must be positive")
	}

	report, err := w.generator.Generate(ctx, input)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(w.storagePath, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create coaching report directory: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", input.MatchID))
	if err := writeReportAtomically(path, report); err != nil {
		return "", err
	}
	return path, nil
}

func writeReportAtomically(path string, report MatchCoachingReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode coaching report: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".report-*.tmp")
	if err != nil {
		return fmt.Errorf("create coaching report temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write coaching report temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync coaching report temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close coaching report temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace coaching report: %w", err)
	}
	return nil
}
