package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ap0calypse644/git_gud/internal/detector"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-detect:", err)
		os.Exit(1)
	}
}

func run() error {
	timelinePath := flag.String("timeline", "", "path to an existing timeline JSON")
	analysis := flag.String("analysis", "deaths", "analysis to run: deaths | fights | waves | all")
	flag.Parse()
	if *timelinePath == "" {
		return fmt.Errorf("-timeline is required")
	}

	f, err := os.Open(*timelinePath)
	if err != nil {
		return fmt.Errorf("open timeline: %w", err)
	}
	defer f.Close()

	var tl timeline.MatchTimeline
	if err := json.NewDecoder(f).Decode(&tl); err != nil {
		return fmt.Errorf("decode timeline: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	var output any
	switch *analysis {
	case "deaths":
		output = detector.AnalyzeDeaths(&tl)
	case "fights":
		output = detector.AnalyzeFights(&tl)
	case "waves":
		output = detector.AnalyzePostWaves(&tl)
	case "all":
		output = struct {
			Deaths detector.Analysis          `json:"deaths"`
			Fights detector.FightAnalysis     `json:"fights"`
			Waves  detector.PostWaveAnalysis  `json:"waves"`
		}{
			Deaths: detector.AnalyzeDeaths(&tl),
			Fights: detector.AnalyzeFights(&tl),
			Waves:  detector.AnalyzePostWaves(&tl),
		}
	default:
		return fmt.Errorf("invalid -analysis %q: want deaths, fights, waves, or all", *analysis)
	}

	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("encode detector analysis: %w", err)
	}
	return nil
}
