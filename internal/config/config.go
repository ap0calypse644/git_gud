package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

type Config struct {
	Player struct {
		AccountID uint32 `json:"account_id"`
	} `json:"player"`
	OpenDota struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	} `json:"opendota"`
	HTTP struct {
		Timeout Duration `json:"timeout"`
	} `json:"http"`
	Storage struct {
		Path string `json:"path"`
	} `json:"storage"`
	Poll struct {
		Interval Duration `json:"interval"`
	} `json:"poll"`
	Watcher struct {
		BootstrapExisting bool `json:"bootstrap_existing"`
	} `json:"watcher"`
	Replays struct {
		RequestParse     bool     `json:"request_parse"`
		RetryInterval    Duration `json:"retry_interval"`
		RetryMaxInterval Duration `json:"retry_max_interval"`
		RetryFor         Duration `json:"retry_for"`
		KeepCompressed   bool     `json:"keep_compressed"`
		DownloadTimeout  Duration `json:"download_timeout"`
	} `json:"replays"`
	Coaching struct {
		Enabled         bool     `json:"enabled"`
		Model           string   `json:"model"`
		Timeout         Duration `json:"timeout"`
		MaxOutputTokens int      `json:"max_output_tokens"`
	} `json:"coaching"`
	Patterns struct {
		Enabled       bool `json:"enabled"`
		RecentMatches int  `json:"recent_matches"`
	} `json:"patterns"`
}

func defaults() Config {
	var cfg Config
	cfg.OpenDota.BaseURL = "https://api.opendota.com/api"
	cfg.HTTP.Timeout = Duration(30 * time.Second)
	cfg.Storage.Path = "./data"
	cfg.Poll.Interval = Duration(60 * time.Second)
	cfg.Replays.RequestParse = true
	cfg.Replays.RetryInterval = Duration(5 * time.Minute)
	cfg.Replays.RetryMaxInterval = Duration(time.Hour)
	cfg.Replays.RetryFor = Duration(168 * time.Hour)
	cfg.Replays.DownloadTimeout = Duration(10 * time.Minute)
	cfg.Coaching.Enabled = true
	cfg.Coaching.Model = "gpt-5.6-terra"
	cfg.Coaching.Timeout = Duration(90 * time.Second)
	cfg.Coaching.MaxOutputTokens = 3000
	cfg.Patterns.Enabled = true
	cfg.Patterns.RecentMatches = 20
	return cfg
}

func Load(path string) (Config, error) {
	cfg := defaults()

	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if key := strings.TrimSpace(os.Getenv("OPENDOTA_API_KEY")); key != "" {
		cfg.OpenDota.APIKey = key
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error
	if c.Player.AccountID == 0 {
		errs = append(errs, errors.New("player.account_id must be non-zero"))
	}
	if strings.TrimSpace(c.OpenDota.BaseURL) == "" {
		errs = append(errs, errors.New("opendota.base_url must not be empty"))
	}
	if strings.TrimSpace(c.Storage.Path) == "" {
		errs = append(errs, errors.New("storage.path must not be empty"))
	}
	if c.HTTP.Timeout.Duration() <= 0 {
		errs = append(errs, errors.New("http.timeout must be positive"))
	}
	if c.Poll.Interval.Duration() <= 0 {
		errs = append(errs, errors.New("poll.interval must be positive"))
	}
	if c.Replays.RetryInterval.Duration() <= 0 {
		errs = append(errs, errors.New("replays.retry_interval must be positive"))
	}
	if c.Replays.RetryMaxInterval.Duration() <= 0 {
		errs = append(errs, errors.New("replays.retry_max_interval must be positive"))
	} else if c.Replays.RetryMaxInterval.Duration() < c.Replays.RetryInterval.Duration() {
		errs = append(errs, errors.New("replays.retry_max_interval must be greater than or equal to replays.retry_interval"))
	}
	if c.Replays.RetryFor.Duration() <= 0 {
		errs = append(errs, errors.New("replays.retry_for must be positive"))
	}
	if c.Replays.DownloadTimeout.Duration() <= 0 {
		errs = append(errs, errors.New("replays.download_timeout must be positive"))
	}
	if c.Coaching.Enabled {
		if strings.TrimSpace(c.Coaching.Model) == "" {
			errs = append(errs, errors.New("coaching.model must not be empty when coaching is enabled"))
		}
		if c.Coaching.Timeout.Duration() <= 0 {
			errs = append(errs, errors.New("coaching.timeout must be positive when coaching is enabled"))
		}
		if c.Coaching.MaxOutputTokens <= 0 {
			errs = append(errs, errors.New("coaching.max_output_tokens must be positive when coaching is enabled"))
		}
	}
	if c.Patterns.Enabled && c.Patterns.RecentMatches <= 0 {
		errs = append(errs, errors.New("patterns.recent_matches must be positive when patterns are enabled"))
	}
	return errors.Join(errs...)
}
