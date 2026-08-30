# Local diagnostics

`git_gud` provides a read-only local consistency check:

```bash
go run ./cmd/git-gud -config config.json -doctor
```

The command loads `state.json` and persisted artifacts only. It exits before constructing OpenDota, replay-download, watcher, or OpenAI clients, performs no network calls, and does not repair or rewrite state.

It checks:

- state map keys and known match statuses;
- `replay_downloaded` matches have a usable Source 2 replay header;
- later matches with a replay cache reference report missing/invalid cache files as warnings rather than hard failures;
- `timeline_ready` and `coaching_ready` matches have decodable timelines whose match/account identity matches state/config;
- `coaching_ready` matches have decodable reports with the expected match ID;
- matches marked `pattern_recorded` exist in `patterns.json`;
- persisted artifact paths that contradict an earlier state are surfaced as warnings.

A clean run prints `doctor_status: ok` and exits successfully. Warnings also exit successfully. Consistency errors print all findings and return a non-zero exit status so the command can be used in scripts.

The replay check is intentionally header-level. Full replay integrity is still established by the normal timeline parser.
