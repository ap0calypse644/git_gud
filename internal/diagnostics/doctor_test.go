package diagnostics

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/patterns"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestRunAcceptsConsistentCompletedMatch(t *testing.T) {
	root := t.TempDir()
	cfg := diagnosticConfig(root)
	replayPath := filepath.Join(root, "replays", "42.dem")
	timelinePath := filepath.Join(root, "timelines", "42.json")
	reportPath := filepath.Join(root, "reports", "42.json")
	writeFile(t, replayPath, append(append([]byte(nil), source2ReplayMagic...), []byte("payload")...))
	writeJSON(t, timelinePath, timeline.MatchTimeline{MatchID: 42, AccountID: cfg.Player.AccountID})
	writeJSON(t, reportPath, coaching.MatchCoachingReport{MatchID: 42, Hero: "slark", Summary: "summary"})
	if _, err := patterns.NewStore(root, cfg.Patterns.RecentMatches).Record(coaching.MatchCoachingInput{MatchID: 42, Hero: "slark"}); err != nil {
		t.Fatal(err)
	}

	state := storage.NewState()
	state.Put(&storage.MatchState{
		MatchID:         42,
		Status:          storage.StatusCoachingReady,
		ReplayPath:      replayPath,
		TimelinePath:    timelinePath,
		ReportPath:      reportPath,
		PatternRecorded: true,
	})

	report := Run(cfg, state)
	if report.Errors != 0 || report.Warnings != 0 || report.CheckedMatches != 1 {
		t.Fatalf("report=%#v", report)
	}
}

func TestRunFindsRequiredArtifactAndPatternMismatches(t *testing.T) {
	root := t.TempDir()
	cfg := diagnosticConfig(root)
	timelinePath := filepath.Join(root, "timelines", "11.json")
	reportPath := filepath.Join(root, "reports", "11.json")
	writeJSON(t, timelinePath, timeline.MatchTimeline{MatchID: 11, AccountID: cfg.Player.AccountID})
	writeJSON(t, reportPath, coaching.MatchCoachingReport{MatchID: 99})

	state := storage.NewState()
	state.Put(&storage.MatchState{MatchID: 10, Status: storage.StatusTimelineReady, TimelinePath: filepath.Join(root, "timelines", "missing.json")})
	state.Put(&storage.MatchState{MatchID: 11, Status: storage.StatusCoachingReady, TimelinePath: timelinePath, ReportPath: reportPath, PatternRecorded: true})

	report := Run(cfg, state)
	if report.Errors != 3 {
		t.Fatalf("errors=%d issues=%#v", report.Errors, report.Issues)
	}
	joined := issuesText(report)
	for _, want := range []string{"open timeline", "report match_id 99", "patterns.json has no record"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("issues missing %q: %s", want, joined)
		}
	}
}

func TestRunWarnsForMissingReplayCacheAfterTimelineReady(t *testing.T) {
	root := t.TempDir()
	cfg := diagnosticConfig(root)
	timelinePath := filepath.Join(root, "timelines", "77.json")
	writeJSON(t, timelinePath, timeline.MatchTimeline{MatchID: 77, AccountID: cfg.Player.AccountID})

	state := storage.NewState()
	state.Put(&storage.MatchState{
		MatchID:      77,
		Status:       storage.StatusTimelineReady,
		ReplayPath:   filepath.Join(root, "replays", "77.dem"),
		TimelinePath: timelinePath,
	})

	report := Run(cfg, state)
	if report.Errors != 0 || report.Warnings != 1 {
		t.Fatalf("report=%#v", report)
	}
	if report.Issues[0].Artifact != "replay" {
		t.Fatalf("issue=%#v", report.Issues[0])
	}
}

func TestWriteSummarizesIssues(t *testing.T) {
	report := Report{
		CheckedMatches: 2,
		Errors:         1,
		Warnings:       1,
		Issues: []Issue{
			{Severity: SeverityError, MatchID: 10, Artifact: "timeline", Message: "missing"},
			{Severity: SeverityWarning, Artifact: "patterns", Message: "stale"},
		},
	}
	var out bytes.Buffer
	if err := Write(&out, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"doctor_status: error", "checked_matches: 2", "ERROR match=10 artifact=timeline", "WARNING artifact=patterns"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func diagnosticConfig(root string) config.Config {
	cfg := config.Config{}
	cfg.Storage.Path = root
	cfg.Player.AccountID = 256161923
	cfg.Patterns.RecentMatches = 20
	return cfg
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, data)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func issuesText(report Report) string {
	var out strings.Builder
	for _, issue := range report.Issues {
		out.WriteString(issue.Message)
		out.WriteByte('\n')
	}
	return out.String()
}
