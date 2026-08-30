package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{
  "player": {"account_id": 256161923},
  "poll": {"interval": "15s"}
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENDOTA_API_KEY", "from-env")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Player.AccountID != 256161923 {
		t.Fatalf("account id = %d", cfg.Player.AccountID)
	}
	if cfg.Poll.Interval.Duration() != 15*time.Second {
		t.Fatalf("poll interval = %s", cfg.Poll.Interval.Duration())
	}
	if cfg.HTTP.Timeout.Duration() != 30*time.Second {
		t.Fatalf("http timeout = %s", cfg.HTTP.Timeout.Duration())
	}
	if cfg.Replays.RetryInterval.Duration() != 5*time.Minute {
		t.Fatalf("retry interval = %s", cfg.Replays.RetryInterval.Duration())
	}
	if cfg.Replays.RetryMaxInterval.Duration() != time.Hour {
		t.Fatalf("retry max interval = %s", cfg.Replays.RetryMaxInterval.Duration())
	}
	if cfg.OpenDota.APIKey != "from-env" {
		t.Fatalf("api key = %q", cfg.OpenDota.APIKey)
	}
	if !cfg.Coaching.Enabled {
		t.Fatal("coaching should be enabled by default")
	}
	if cfg.Coaching.Model != "gpt-5.6-terra" {
		t.Fatalf("coaching model = %q", cfg.Coaching.Model)
	}
	if cfg.Coaching.Timeout.Duration() != 90*time.Second {
		t.Fatalf("coaching timeout = %s", cfg.Coaching.Timeout.Duration())
	}
	if cfg.Coaching.MaxOutputTokens != 3000 {
		t.Fatalf("coaching max output tokens = %d", cfg.Coaching.MaxOutputTokens)
	}
	if !cfg.Patterns.Enabled {
		t.Fatal("patterns should be enabled by default")
	}
	if cfg.Patterns.RecentMatches != 20 {
		t.Fatalf("patterns recent matches = %d", cfg.Patterns.RecentMatches)
	}
}

func TestValidateRejectsRetryMaxBelowBase(t *testing.T) {
	cfg := defaults()
	cfg.Player.AccountID = 1
	cfg.Replays.RetryInterval = Duration(10 * time.Minute)
	cfg.Replays.RetryMaxInterval = Duration(5 * time.Minute)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected retry max validation error")
	}
}
