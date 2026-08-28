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
	"syscall"

	"github.com/ap0calypse644/git_gud/internal/config"
	"github.com/ap0calypse644/git_gud/internal/opendota"
	"github.com/ap0calypse644/git_gud/internal/replay"
	"github.com/ap0calypse644/git_gud/internal/storage"
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
	once := flag.Bool("once", false, "run one discovery/replay-acquisition cycle and exit")
	matchID := flag.Int64("match", 0, "process one match ID immediately (useful for historical testing)")
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
	store := storage.New(filepath.Join(cfg.Storage.Path, "state.json"))
	service := watcher.New(cfg, api, downloader, store, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *matchID > 0 {
		return service.ProcessMatch(ctx, *matchID)
	}
	if *once {
		return service.RunOnce(ctx)
	}
	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
