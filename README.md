# git_gud

`git_gud` is a personal Dota 2 replay-coaching service. It automatically discovers new matches for one configured OpenDota account, acquires and parses the replay, derives causal detector evidence, builds a compact coaching input, and generates a structured AI coaching report.

The AI boundary is intentionally strict: the remote report generator receives only `MatchCoachingInput`, never the raw `MatchTimeline`.

## Requirements

- Go 1.23+
- A public OpenDota profile/match history
- Internet access to OpenDota, Valve replay hosts, and the configured OpenAI API endpoint
- `OPENAI_API_KEY` when coaching is enabled

An OpenDota API key is optional for the single-player use case and can be supplied in config or via `OPENDOTA_API_KEY`.

The OpenAI key is environment-only and is not stored in `config.json`.

## Quick start

```bash
cp config.example.json config.json
```

Set your OpenAI API key in the shell, then run:

```bash
go run ./cmd/git-gud -config config.json
```

The example config is set to account `256161923` and has automatic coaching enabled.

On the first normal watcher run, `bootstrap_existing: false` establishes the newest current match as the discovery baseline and does not pull older matches automatically. Every subsequently discovered match is persisted to `data/state.json` and advanced through the processing pipeline.

## Manual historical analysis

A historical match can be processed without changing the watcher's discovery baseline:

```bash
go run ./cmd/git-gud -config config.json -match 8962145737
```

The manual and automatic paths use the same processor. With coaching enabled, both run:

```text
match metadata
  -> replay acquisition
  -> deterministic timeline
  -> compact MatchCoachingInput
  -> structured AI coaching report
```

Reports are stored at:

```text
data/reports/<match_id>.json
```

Timelines are stored at:

```text
data/timelines/<match_id>.json
```

To perform one automatic discovery/processing cycle and exit:

```bash
go run ./cmd/git-gud -config config.json -once
```

## State machine

```text
discovered
    |
    v
metadata_ready
    |
    +-- replay metadata missing --> replay_waiting
    |                                |
    |                         request OpenDota parse
    |                                |
    +--------------------------------+
    |
    v
replay_downloaded
    |
    v
timeline_ready
    |
    v
coaching_ready
```

If replay acquisition exceeds the configured retry window, the match becomes `replay_unavailable` and is no longer retried automatically.

`timeline_ready` is deliberately durable. If the coaching provider fails, the watcher retries from the existing timeline rather than re-downloading or re-parsing the replay.

State and report artifacts are written atomically.

## Configuration

See [`config.example.json`](config.example.json). Important defaults:

- polling: `60s`
- replay retry interval: `5m`
- replay retry window: `168h`
- replay download timeout: `10m`
- bootstrap historical matches: disabled
- compressed replay retention: disabled
- coaching: enabled
- coaching model: `gpt-5.6-terra`
- coaching timeout: `90s`
- coaching max output tokens: `3000`

OpenAI environment variables:

- `OPENAI_API_KEY`: required when `coaching.enabled` is true
- `OPENAI_MODEL`: optional model override
- `OPENAI_BASE_URL`: optional API base URL override

OpenDota environment variable:

- `OPENDOTA_API_KEY`: optional API-key override

## Coaching behavior

The deterministic layer emits low-confidence review candidates and compact evidence. The report layer is expected to:

- prioritize only a handful of high-value decisions;
- preserve detector confidence;
- distinguish decision-time facts from retrospective outcomes;
- keep inference separate from deterministic facts;
- avoid hidden-information claims and exact unseen enemy positions;
- group overlapping detector signals rather than duplicate coaching;
- suggest plausible alternatives without hindsight-only reasoning.

Detector candidates are review targets, not automatic proof of mistakes.

## Tests

```bash
go test ./...
```

CI also runs `go vet ./...`. Unit tests use local HTTP servers and fakes; they do not call OpenDota, Valve, or OpenAI.
