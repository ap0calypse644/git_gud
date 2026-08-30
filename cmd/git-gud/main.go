package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/opsview"
	"github.com/ap0calypse644/git_gud/internal/patterns"
	"github.com/ap0calypse644/git_gud/internal/processor"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
	"github.com/ap0calypse644/git_gud/internal/watcher"
)

type missingAPIKeyCoach struct{}

func (missingAPIKeyCoach) Generate(context.Context, coaching.MatchCoachingInput) (string, error) {
	return "", errors.New("OPENAI_API_KEY is required to generate a coaching report")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.json", "path to JSON config")
	once := flag.Bool("once", false, "run one automatic discovery/processing cycle and exit")
	matchID := flag.Int64("match", 0, "process one match ID immediately, including historical matches")
	showStatus := flag.Bool("status", false, "print watcher and retry status without network calls")
	historyLimit := flag.Int("history", 0, "print the most recent N tracked matches without network calls")
	reportMatchID := flag.Int64("report", 0, "print the persisted coaching report for one tracked match ID")
	flag.Parse()

	if *historyLimit < 0 {
		return errors.New("-history must be positive")
	}
	if *reportMatchID < 0 {
		return errors.New("-report must be a positive match ID")
	}
	modes := 0
	for _, selected := range []bool{*once, *matchID > 0, *showStatus, *historyLimit > 0, *reportMatchID > 0} {
		if selected {
			modes++
		}
	}
	if modes > 1 {
		return errors.New("choose only one of -once, -match, -status, -history, or -report")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store := storage.New(filepath.Join(cfg.Storage.Path, "state.json"))

	// Operational inspection modes are deliberately read-only and exit before
	// OpenDota, replay, watcher, or OpenAI clients are constructed.
	if *showStatus || *historyLimit > 0 || *reportMatchID > 0 {
		state, err := store.Load()
		if err != nil {
			return err
		}
		switch {
		case *showStatus:
			snapshot := opsview.BuildStatus(
				state,
				cfg.Replays.RetryInterval.Duration(),
				cfg.Coaching.Enabled,
				cfg.Patterns.Enabled,
				time.Now().UTC(),
			)
			return opsview.WriteStatus(os.Stdout, snapshot)
		case *historyLimit > 0:
			entries, err := opsview.RecentHistory(state, *historyLimit)
			if err != nil {
				return err
			}
			return opsview.WriteHistory(os.Stdout, entries)
		case *reportMatchID > 0:
			match := state.Match(*reportMatchID)
			if match == nil {
				return fmt.Errorf("match %d is not tracked", *reportMatchID)
			}
			if strings.TrimSpace(match.ReportPath) == "" {
				return fmt.Errorf("match %d has no persisted coaching report", *reportMatchID)
			}
			report, err := opsview.LoadReport(match.ReportPath)
			if err != nil {
				return err
			}
			return opsview.WriteReport(os.Stdout, report)
		}
	}

	if err := os.MkdirAll(cfg.Storage.Path, 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiHTTPClient := &http.Client{Timeout: cfg.HTTP.Timeout.Duration()}
	replayHTTPClient := &http.Client{Timeout: cfg.Replays.DownloadTimeout.Duration()}
	api := opendota.NewClient(cfg.OpenDota.BaseURL, cfg.OpenDota.APIKey, apiHTTPClient)
	downloader := replay.NewDownloader(replayHTTPClient, cfg.Storage.Path, cfg.Replays.KeepCompressed)
	timelineBuilder := timeline.NewBuilder(cfg.Storage.Path, cfg.Player.AccountID)

	var coach processor.Coach
	if cfg.Coaching.Enabled {
		apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if apiKey == "" {
			// Keep the coaching stage configured so a match that actually needs a
			// report fails clearly, but do not block Phase H pattern backfills for
			// already-coached matches that never invoke the provider.
			coach = missingAPIKeyCoach{}
		} else {
			model := cfg.Coaching.Model
			if envModel := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); envModel != "" {
				model = envModel
			}
			reporter := coaching.NewOpenAIReporter(
				apiKey,
				model,
				&http.Client{Timeout: cfg.Coaching.Timeout.Duration()},
			)
			if baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); baseURL != "" {
				reporter.BaseURL = baseURL
			}
			reporter.MaxOutputTokens = cfg.Coaching.MaxOutputTokens
			coach = coaching.NewReportArtifactWriter(cfg.Storage.Path, reporter)
		}
	}

	var patternRecorder processor.PatternRecorder
	if cfg.Patterns.Enabled {
		patternRecorder = patterns.NewStore(cfg.Storage.Path, cfg.Patterns.RecentMatches)
	}

	matchProcessor := processor.NewWithCoachAndPatterns(cfg, api, downloader, timelineBuilder, coach, patternRecorder, store, logger)
	watchService := watcher.New(cfg, api, matchProcessor, store, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *matchID > 0 {
		result, err := matchProcessor.Process(ctx, *matchID, true)
		if err != nil {
			return err
		}
		logger.Info("match processing status", "match_id", *matchID, "status", result.Status, "report_path", result.ReportPath)
		return nil
	}
	if *once {
		return watchService.RunOnce(ctx)
	}
	if err := watchService.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
