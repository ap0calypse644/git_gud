package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ap0calypse644/git_gud/internal/coaching"
	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-gud-coach:", err)
		os.Exit(1)
	}
}

func run() error {
	timelinePath := flag.String("timeline", "", "path to an existing timeline JSON")
	model := flag.String("model", strings.TrimSpace(os.Getenv("OPENAI_MODEL")), "OpenAI model (default: gpt-5.6-terra)")
	timeout := flag.Duration("timeout", 90*time.Second, "OpenAI request timeout")
	maxOutputTokens := flag.Int("max-output-tokens", 3000, "maximum report output tokens")
	flag.Parse()

	if strings.TrimSpace(*timelinePath) == "" {
		return fmt.Errorf("-timeline is required")
	}
	if *timeout <= 0 {
		return fmt.Errorf("-timeout must be positive")
	}
	if *maxOutputTokens <= 0 {
		return fmt.Errorf("-max-output-tokens must be positive")
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
	input := coaching.BuildMatchCoachingInput(&tl)

	reporter := coaching.NewOpenAIReporter(
		strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		strings.TrimSpace(*model),
		&http.Client{Timeout: *timeout},
	)
	if baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); baseURL != "" {
		reporter.BaseURL = baseURL
	}
	reporter.MaxOutputTokens = *maxOutputTokens

	report, err := reporter.Generate(context.Background(), input)
	if err != nil {
		return fmt.Errorf("generate coaching report: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encode coaching report: %w", err)
	}
	return nil
}
