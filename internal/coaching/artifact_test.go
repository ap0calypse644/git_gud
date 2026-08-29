package coaching

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fakeReportGenerator struct {
	inputs []MatchCoachingInput
	report MatchCoachingReport
	err    error
}

func (f *fakeReportGenerator) Generate(_ context.Context, input MatchCoachingInput) (MatchCoachingReport, error) {
	f.inputs = append(f.inputs, input)
	return f.report, f.err
}

func TestReportArtifactWriterPersistsStructuredReport(t *testing.T) {
	root := t.TempDir()
	generator := &fakeReportGenerator{report: MatchCoachingReport{
		MatchID: 42,
		Hero:    "slark",
		Summary: "Review one decision.",
		Moments: []CoachingReportMoment{},
	}}
	writer := NewReportArtifactWriter(root, generator)
	input := MatchCoachingInput{MatchID: 42, Hero: "slark", Moments: []CoachingMoment{}}

	path, err := writer.Generate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, "reports", "42.json")
	if path != wantPath {
		t.Fatalf("path=%q, want %q", path, wantPath)
	}
	if len(generator.inputs) != 1 || generator.inputs[0].MatchID != 42 {
		t.Fatalf("generator inputs=%#v", generator.inputs)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got MatchCoachingReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.MatchID != 42 || got.Hero != "slark" || got.Summary != "Review one decision." {
		t.Fatalf("report=%#v", got)
	}
}
