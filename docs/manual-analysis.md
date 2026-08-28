# Manual historical analysis

`git_gud` supports two equivalent ways to enter the match-processing pipeline:

1. automatic discovery from the configured OpenDota account; and
2. explicit processing of a historical match ID.

The explicit CLI form is:

```bash
go run ./cmd/git-gud -config config.json -match <match_id>
```

This is a permanent user-facing capability, not a test-only path. As later milestones add replay parsing, deterministic decision extraction, and coaching, the same command will run the complete pipeline for the supplied match ID.

Manual processing must not advance or otherwise alter the automatic watcher's discovery baseline. It may persist per-match acquisition/parsing/analysis state so repeated requests can reuse already downloaded or generated artifacts.

Both entry paths must converge on the same match processor so analysis behavior cannot diverge between historical and newly discovered matches.
