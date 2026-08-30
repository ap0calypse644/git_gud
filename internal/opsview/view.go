package opsview

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/storage"
)

type PendingMatch struct {
	MatchID     int64
	Status      storage.MatchStatus
	NextRetryAt *time.Time
	LastError   string
}

type StatusSnapshot struct {
	Initialized      bool
	LastSeenMatchID  int64
	TrackedMatches   int
	PendingMatches   int
	PatternRecorded  int
	StatusCounts     map[storage.MatchStatus]int
	Pending          []PendingMatch
}

type HistoryEntry struct {
	MatchID         int64
	StartTime       int64
	Status          storage.MatchStatus
	Hero            string
	Summary         string
	ReportPath      string
	PatternRecorded bool
	LastError       string
}

func BuildStatus(state storage.State, retryInterval time.Duration, coachingEnabled, patternsEnabled bool, now time.Time) StatusSnapshot {
	snapshot := StatusSnapshot{
		Initialized:     state.Initialized,
		LastSeenMatchID: state.LastSeenMatchID,
		TrackedMatches:  len(state.Matches),
		StatusCounts:    make(map[storage.MatchStatus]int),
		Pending:         []PendingMatch{},
	}
	for _, match := range state.Matches {
		if match == nil {
			continue
		}
		snapshot.StatusCounts[match.Status]++
		if match.PatternRecorded {
			snapshot.PatternRecorded++
		}
		if !isPending(match, coachingEnabled, patternsEnabled) {
			continue
		}
		pending := PendingMatch{MatchID: match.MatchID, Status: match.Status, LastError: match.LastError}
		if match.LastAttemptAt != nil && retryInterval > 0 {
			retryAt := match.LastAttemptAt.Add(retryInterval)
			if retryAt.After(now) {
				pending.NextRetryAt = &retryAt
			}
		}
		snapshot.Pending = append(snapshot.Pending, pending)
	}
	snapshot.PendingMatches = len(snapshot.Pending)
	sort.Slice(snapshot.Pending, func(i, j int) bool { return snapshot.Pending[i].MatchID < snapshot.Pending[j].MatchID })
	return snapshot
}

func WriteStatus(w io.Writer, snapshot StatusSnapshot) error {
	if w == nil {
		return errors.New("status writer is nil")
	}
	fmt.Fprintf(w, "watcher_initialized: %t\n", snapshot.Initialized)
	fmt.Fprintf(w, "last_seen_match_id: %d\n", snapshot.LastSeenMatchID)
	fmt.Fprintf(w, "tracked_matches: %d\n", snapshot.TrackedMatches)
	fmt.Fprintf(w, "pending_matches: %d\n", snapshot.PendingMatches)
	fmt.Fprintf(w, "pattern_recorded_matches: %d\n", snapshot.PatternRecorded)

	statuses := make([]string, 0, len(snapshot.StatusCounts))
	for status := range snapshot.StatusCounts {
		statuses = append(statuses, string(status))
	}
	sort.Strings(statuses)
	for _, raw := range statuses {
		fmt.Fprintf(w, "status.%s: %d\n", raw, snapshot.StatusCounts[storage.MatchStatus(raw)])
	}

	for _, pending := range snapshot.Pending {
		retry := "ready"
		if pending.NextRetryAt != nil {
			retry = pending.NextRetryAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(w, "pending.%d: status=%s retry=%s", pending.MatchID, pending.Status, retry)
		if strings.TrimSpace(pending.LastError) != "" {
			fmt.Fprintf(w, " error=%q", pending.LastError)
		}
		fmt.Fprintln(w)
	}
	return nil
}

func RecentHistory(state storage.State, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		return nil, errors.New("history limit must be positive")
	}
	matches := make([]*storage.MatchState, 0, len(state.Matches))
	for _, match := range state.Matches {
		if match != nil {
			matches = append(matches, match)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].StartTime != matches[j].StartTime {
			return matches[i].StartTime > matches[j].StartTime
		}
		return matches[i].MatchID > matches[j].MatchID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}

	entries := make([]HistoryEntry, 0, len(matches))
	for _, match := range matches {
		entry := HistoryEntry{
			MatchID:         match.MatchID,
			StartTime:       match.StartTime,
			Status:          match.Status,
			ReportPath:      match.ReportPath,
			PatternRecorded: match.PatternRecorded,
			LastError:       match.LastError,
		}
		if match.ReportPath != "" {
			report, err := LoadReport(match.ReportPath)
			if err != nil {
				entry.Summary = "report unavailable: " + err.Error()
			} else {
				entry.Hero = report.Hero
				entry.Summary = report.Summary
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func WriteHistory(w io.Writer, entries []HistoryEntry) error {
	if w == nil {
		return errors.New("history writer is nil")
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "MATCH_ID\tSTART\tSTATUS\tHERO\tPATTERN\tREPORT")
	for _, entry := range entries {
		start := "-"
		if entry.StartTime > 0 {
			start = time.Unix(entry.StartTime, 0).UTC().Format("2006-01-02 15:04")
		}
		pattern := "no"
		if entry.PatternRecorded {
			pattern = "yes"
		}
		report := entry.ReportPath
		if report == "" {
			report = "-"
		}
		hero := entry.Hero
		if hero == "" {
			hero = "-"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", entry.MatchID, start, entry.Status, hero, pattern, report)
		if strings.TrimSpace(entry.Summary) != "" {
			fmt.Fprintf(tw, "\t\t\t\t\tsummary: %s\n", oneLine(entry.Summary))
		}
		if strings.TrimSpace(entry.LastError) != "" {
			fmt.Fprintf(tw, "\t\t\t\t\terror: %s\n", oneLine(entry.LastError))
		}
	}
	return tw.Flush()
}

func LoadReport(path string) (coaching.MatchCoachingReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return coaching.MatchCoachingReport{}, fmt.Errorf("read report: %w", err)
	}
	var report coaching.MatchCoachingReport
	if err := json.Unmarshal(data, &report); err != nil {
		return coaching.MatchCoachingReport{}, fmt.Errorf("decode report: %w", err)
	}
	return report, nil
}

func WriteReport(w io.Writer, report coaching.MatchCoachingReport) error {
	if w == nil {
		return errors.New("report writer is nil")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func isPending(match *storage.MatchState, coachingEnabled, patternsEnabled bool) bool {
	if match == nil || match.Status == storage.StatusReplayUnavailable {
		return false
	}
	if match.Status == storage.StatusCoachingReady {
		return patternsEnabled && !match.PatternRecorded
	}
	if match.Status == storage.StatusTimelineReady {
		return coachingEnabled || (patternsEnabled && !match.PatternRecorded)
	}
	return true
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
