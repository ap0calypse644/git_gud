package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-coaching-input:", err)
		os.Exit(1)
	}
}

func run() error {
	timelinePath := flag.String("timeline", "", "path to an existing timeline JSON")
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
	if err := enc.Encode(coaching.BuildMatchCoachingInput(&tl)); err != nil {
		return fmt.Errorf("encode coaching input: %w", err)
	}
	return nil
}
