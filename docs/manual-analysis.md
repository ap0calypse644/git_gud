# Manual historical analysis

`git_gud` supports two equivalent ways to enter the match-processing pipeline:

1. automatic discovery from the configured OpenDota account; and
2. explicit processing of a historical match ID.

The explicit CLI form is:

```bash
go run ./cmd/git-gud -config config.json -match <match_id>
```

This is a permanent user-facing capability, not a test-only path. With coaching and recurring patterns enabled, the command runs the complete pipeline for the supplied match ID:

```text
match metadata
  -> replay acquisition
  -> deterministic timeline
  -> compact MatchCoachingInput
  -> recurring-pattern facts
  -> structured AI coaching report
```

The generated report is persisted at:

```text
<storage.path>/reports/<match_id>.json
```

Detector-normalized cross-match history is persisted at:

```text
<storage.path>/patterns.json
```

Manual processing must not advance or otherwise alter the automatic watcher's discovery baseline. It may persist per-match acquisition/parsing/analysis state so repeated requests can reuse already downloaded or generated artifacts.

Both entry paths converge on the same match processor so analysis behavior cannot diverge between historical and newly discovered matches. Pattern recording is idempotent by match ID, so reprocessing a historical match replaces its normalized pattern record rather than double-counting it.

`OPENAI_API_KEY` is required only when the invocation actually needs to generate a new coaching report. Pattern-only backfill of an already-coached match does not require the key. The key is never stored in `config.json`. `OPENAI_MODEL` and `OPENAI_BASE_URL` may optionally override the configured model and default API base URL.
