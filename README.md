# git_gud

`git_gud` is a personal Dota 2 replay-coaching service. The end goal is to turn each completed match into a player-centric timeline and generate coaching such as: "at 13:45, taking the next wave was unsafe because the relevant enemy catch heroes had been missing and no ally was close enough to support you."

This branch implements the first two milestones only:

- **M0:** detect new public matches for a configured OpenDota account and persist durable watcher state.
- **M1:** fetch detailed OpenDota match metadata, request parsing when replay metadata is missing, download the Valve replay when it becomes available, and decompress it to a local `.dem` file.

Replay parsing and coaching are deliberately *not* in M0/M1. M2 will feed the `.dem` into a Source 2 replay parser (planned: Manta) and produce normalized timeline data.

## Requirements

- Go 1.23+
- A public OpenDota profile/match history
- Internet access to `api.opendota.com` and Valve replay hosts

An OpenDota API key is optional for the initial single-player use case, but can be supplied in config or via `OPENDOTA_API_KEY`.

## Quick start

```bash
cp config.example.json config.json
go run ./cmd/git-gud -config config.json
```

The example config is already set to account `256161923`.

On its **first normal watcher run**, `bootstrap_existing: false` establishes the newest current match as the baseline and does not download historical replays. Every subsequently discovered match is persisted to `data/state.json` and processed.

For a one-off historical test without changing that baseline:

```bash
go run ./cmd/git-gud -config config.json -match 8962145737
```

To perform one discovery/acquisition cycle and exit:

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
```

If the configured replay retry window expires, the match is marked `replay_unavailable` so the watcher does not hammer OpenDota forever.

State is written atomically to `data/state.json`. Replay downloads are also written through temporary files and atomically renamed only after successful completion.

## Replay acquisition

The service prefers `replay_url` returned by OpenDota. When only `cluster` and `replay_salt` are available, it constructs the standard Valve replay URL:

```text
https://replay<cluster>.valve.net/570/<match_id>_<replay_salt>.dem.bz2
```

The compressed replay is downloaded and then decompressed with Go's standard-library bzip2 reader:

```text
data/replays/<match_id>.dem
```

Set `replays.keep_compressed: true` to retain the `.dem.bz2` alongside it.

## Why OpenDota parsing is requested

A public match can appear in recent-match history before OpenDota has Game Coordinator/replay data for it. In that state there is no reliable replay salt from the normal public match summary. With `replays.request_parse: true`, `git_gud` submits `POST /request/{match_id}` once and waits for replay metadata to appear rather than guessing a replay URL.

## Configuration

See [`config.example.json`](config.example.json). Important defaults:

- polling: `60s`
- replay retry interval: `5m`
- replay retry window: `168h`
- replay download timeout: `10m`
- bootstrap historical matches: disabled
- compressed replay retention: disabled

## Tests

```bash
go test ./...
```

The tests use local HTTP servers/fakes; they do not call OpenDota or Valve.

## Next milestone: M2

M2 will parse downloaded `.dem` files and extract a normalized player-centric event stream. Initial targets:

- hero positions and movement
- deaths/kills/assists
- item and ability events
- combat events and teamfights
- towers/Roshan/objectives
- creep waves and lane exposure
- conservative last-seen enemy state for fog-of-war-safe coaching

The AI layer comes only after this deterministic extraction step.
