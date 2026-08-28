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
	if cfg.OpenDota.APIKey != "from-env" {
		t.Fatalf("api key = %q", cfg.OpenDota.APIKey)
	}
}
