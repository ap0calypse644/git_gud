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

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/patterns"
	"github.com/ap0calypse644/git_gud/internal/processor"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
	"github.com/ap0calypse644/git_gud/internal/timeline"
	"github.com/ap0calypse644/git_gud/internal/watcher"
)

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
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
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
	store := storage.New(filepath.Join(cfg.Storage.Path, "state.json"))

	var coach processor.Coach
	if cfg.Coaching.Enabled {
		apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY is required when coaching.enabled is true")
		}
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

	var patternRecorder processor.PatternRecorder
	if cfg.Patterns.Enabled {
		patternRecorder = patterns.NewStore(cfg.Storage.Path, cfg.Patterns.RecentMatches)
	}

	matchProcessor := processor.NewWithCoachAndPatterns(cfg, api, downloader, timelineBuilder, coach, patternRecorder, store, logger)
	watchService := watcher.New(cfg, api, matchProcessor, store, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *matchID > 0 {
		_, err := matchProcessor.Process(ctx, *matchID, true)
		return err
	}
	if *once {
		return watchService.RunOnce(ctx)
	}
	if err := watchService.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
