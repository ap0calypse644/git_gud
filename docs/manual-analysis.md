# Manual historical analysis

`git_gud` supports two equivalent ways to enter the match-processing pipeline:

1. automatic discovery from the configured OpenDota account; and
2. explicit processing of a historical match ID.

The explicit CLI form is:

```bash
go run ./cmd/git-gud -config config.json -match <match_id>
```

This is a permanent user-facing capability, not a test-only path. With coaching enabled, the command runs the complete pipeline for the supplied match ID:

```text
match metadata
  -> replay acquisition
  -> deterministic timeline
  -> compact MatchCoachingInput
  -> structured AI coaching report
```

The generated report is persisted at:

```text
<storage.path>/reports/<match_id>.json
```

Manual processing must not advance or otherwise alter the automatic watcher's discovery baseline. It may persist per-match acquisition/parsing/analysis state so repeated requests can reuse already downloaded or generated artifacts.

Both entry paths converge on the same match processor so analysis behavior cannot diverge between historical and newly discovered matches.

When `coaching.enabled` is true, `OPENAI_API_KEY` must be present in the environment. The key is never stored in `config.json`. `OPENAI_MODEL` and `OPENAI_BASE_URL` may optionally override the configured model and default API base URL.
