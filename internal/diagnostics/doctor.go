package diagnostics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/patterns"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

var source2ReplayMagic = []byte{'P', 'B', 'D', 'E', 'M', 'S', '2', 0}

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
)

type Issue struct {
	Severity Severity
	MatchID  int64
	Artifact string
	Message  string
}

type Report struct {
	CheckedMatches int
	Errors         int
	Warnings       int
	Issues         []Issue
}

// Run checks only durable local state and artifacts. It performs no repairs and
// no network calls, so it is safe to use while the watcher is stopped or when
// upstream services are unavailable.
func Run(cfg config.Config, state storage.State) Report {
	report := Report{}
	patternIDs, patternErr := loadPatternMatchIDs(cfg, state)
	if patternErr != nil {
		addIssue(&report, SeverityError, 0, "patterns", patternErr.Error())
	}

	keys := make([]string, 0, len(state.Matches))
	for key := range state.Matches {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		match := state.Matches[key]
		if match == nil {
			addIssue(&report, SeverityError, 0, "state", fmt.Sprintf("match entry %q is null", key))
			continue
		}
		report.CheckedMatches++
		if parsed, err := strconv.ParseInt(key, 10, 64); err != nil || parsed != match.MatchID {
			addIssue(&report, SeverityError, match.MatchID, "state", fmt.Sprintf("state key %q does not match match_id %d", key, match.MatchID))
		}
		checkMatch(cfg, match, patternIDs, patternErr == nil, &report)
	}

	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity < report.Issues[j].Severity
		}
		if report.Issues[i].MatchID != report.Issues[j].MatchID {
			return report.Issues[i].MatchID < report.Issues[j].MatchID
		}
		if report.Issues[i].Artifact != report.Issues[j].Artifact {
			return report.Issues[i].Artifact < report.Issues[j].Artifact
		}
		return report.Issues[i].Message < report.Issues[j].Message
	})
	return report
}

func Write(w io.Writer, report Report) error {
	if w == nil {
		return fmt.Errorf("diagnostics writer is nil")
	}
	status := "ok"
	if report.Errors > 0 {
		status = "error"
	} else if report.Warnings > 0 {
		status = "warning"
	}
	fmt.Fprintf(w, "doctor_status: %s\n", status)
	fmt.Fprintf(w, "checked_matches: %d\n", report.CheckedMatches)
	fmt.Fprintf(w, "errors: %d\n", report.Errors)
	fmt.Fprintf(w, "warnings: %d\n", report.Warnings)
	for _, issue := range report.Issues {
		if issue.MatchID > 0 {
			fmt.Fprintf(w, "%s match=%d artifact=%s message=%q\n", issue.Severity, issue.MatchID, issue.Artifact, issue.Message)
		} else {
			fmt.Fprintf(w, "%s artifact=%s message=%q\n", issue.Severity, issue.Artifact, issue.Message)
		}
	}
	return nil
}

func checkMatch(cfg config.Config, match *storage.MatchState, patternIDs map[int64]bool, patternHistoryOK bool, report *Report) {
	if !knownStatus(match.Status) {
		addIssue(report, SeverityError, match.MatchID, "state", fmt.Sprintf("unknown status %q", match.Status))
	}

	if match.ReplayPath != "" {
		severity := SeverityWarning
		if match.Status == storage.StatusReplayDownloaded {
			severity = SeverityError
		}
		if err := validateReplayFile(match.ReplayPath); err != nil {
			addIssue(report, severity, match.MatchID, "replay", err.Error())
		}
	} else if match.Status == storage.StatusReplayDownloaded {
		addIssue(report, SeverityError, match.MatchID, "replay", "replay_downloaded state has no replay_path")
	}

	requiresTimeline := match.Status == storage.StatusTimelineReady || match.Status == storage.StatusCoachingReady
	if requiresTimeline {
		if strings.TrimSpace(match.TimelinePath) == "" {
			addIssue(report, SeverityError, match.MatchID, "timeline", fmt.Sprintf("%s state has no timeline_path", match.Status))
		} else if err := validateTimeline(match.TimelinePath, match.MatchID, cfg.Player.AccountID); err != nil {
			addIssue(report, SeverityError, match.MatchID, "timeline", err.Error())
		}
	} else if match.TimelinePath != "" {
		addIssue(report, SeverityWarning, match.MatchID, "timeline", fmt.Sprintf("timeline_path is present while status is %s", match.Status))
	}

	if match.Status == storage.StatusCoachingReady {
		if strings.TrimSpace(match.ReportPath) == "" {
			addIssue(report, SeverityError, match.MatchID, "report", "coaching_ready state has no report_path")
		} else if err := validateReport(match.ReportPath, match.MatchID); err != nil {
			addIssue(report, SeverityError, match.MatchID, "report", err.Error())
		}
	} else if match.ReportPath != "" {
		addIssue(report, SeverityWarning, match.MatchID, "report", fmt.Sprintf("report_path is present while status is %s", match.Status))
	}

	if match.PatternRecorded {
		if !requiresTimeline {
			addIssue(report, SeverityError, match.MatchID, "patterns", fmt.Sprintf("pattern_recorded is true while status is %s", match.Status))
		}
		if patternHistoryOK && !patternIDs[match.MatchID] {
			addIssue(report, SeverityError, match.MatchID, "patterns", "pattern_recorded is true but patterns.json has no record for this match")
		}
	}
}

func loadPatternMatchIDs(cfg config.Config, state storage.State) (map[int64]bool, error) {
	needPatterns := false
	for _, match := range state.Matches {
		if match != nil && match.PatternRecorded {
			needPatterns = true
			break
		}
	}
	path := patterns.NewStore(cfg.Storage.Path, max(cfg.Patterns.RecentMatches, 1)).Path()
	if !needPatterns {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return map[int64]bool{}, nil
		} else if err != nil {
			return nil, fmt.Errorf("stat pattern history: %w", err)
		}
	}

	history, err := patterns.NewStore(cfg.Storage.Path, max(cfg.Patterns.RecentMatches, 1)).Load()
	if err != nil {
		return nil, err
	}
	ids := make(map[int64]bool, len(history.Matches))
	for _, match := range history.Matches {
		if ids[match.MatchID] {
			return nil, fmt.Errorf("pattern history contains duplicate match_id %d", match.MatchID)
		}
		ids[match.MatchID] = true
	}
	return ids, nil
}

func validateReplayFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open replay %q: %w", path, err)
	}
	defer f.Close()
	magic := make([]byte, len(source2ReplayMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read replay header %q: %w", path, err)
	}
	if !bytes.Equal(magic, source2ReplayMagic) {
		return fmt.Errorf("replay %q has invalid Source 2 header % X", path, magic)
	}
	return nil
}

func validateTimeline(path string, matchID int64, accountID uint32) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open timeline %q: %w", path, err)
	}
	defer f.Close()
	var value timeline.MatchTimeline
	if err := json.NewDecoder(f).Decode(&value); err != nil {
		return fmt.Errorf("decode timeline %q: %w", path, err)
	}
	if value.MatchID != matchID {
		return fmt.Errorf("timeline match_id %d does not match state match_id %d", value.MatchID, matchID)
	}
	if accountID > 0 && value.AccountID != accountID {
		return fmt.Errorf("timeline account_id %d does not match configured account_id %d", value.AccountID, accountID)
	}
	return nil
}

func validateReport(path string, matchID int64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read report %q: %w", path, err)
	}
	var value coaching.MatchCoachingReport
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode report %q: %w", path, err)
	}
	if value.MatchID != matchID {
		return fmt.Errorf("report match_id %d does not match state match_id %d", value.MatchID, matchID)
	}
	return nil
}

func knownStatus(status storage.MatchStatus) bool {
	switch status {
	case storage.StatusDiscovered,
		storage.StatusMetadataReady,
		storage.StatusReplayWaiting,
		storage.StatusReplayDownloaded,
		storage.StatusTimelineReady,
		storage.StatusCoachingReady,
		storage.StatusReplayUnavailable:
		return true
	default:
		return false
	}
}

func addIssue(report *Report, severity Severity, matchID int64, artifact, message string) {
	report.Issues = append(report.Issues, Issue{Severity: severity, MatchID: matchID, Artifact: artifact, Message: message})
	if severity == SeverityError {
		report.Errors++
	} else {
		report.Warnings++
	}
}
