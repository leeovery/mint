# Actionable notes-timeout error

During the v0.6.0 release (2026-07-20), notes generation failed twice with exactly this:

```
✗ notes      AI timed out
```

The release was the largest this project has shipped — 75 squashed PRs of diff, even after the repo's `diff_exclude` list and `max_diff_lines = 60000` cap — and the default 60-second per-attempt deadline had no chance against it. The fix turned out to be one line of config: `timeout = 300` under `[release]` in `.mint.toml`. But discovering that took reading `internal/config/config.go`, because nothing user-facing mentions that the knob exists: not the error message, not an obvious config-reference trail from the failure.

The idea is to make the timeout error carry its own remedy. When notes generation dies on the deadline, say so in terms the user can act on: that the per-attempt AI deadline expired, what the current value is and where it came from (default vs configured), and that `timeout = <seconds>` under `[release]` (or the shared top-level key) raises it. A failure that names its own knob converts a source-diving session into a ten-second config edit.

A step further, if it ever earns its keep: the deadline could scale with the work. Mint already knows the diff size it's about to send — it enforces `max_diff_lines` — so a release-day diff near the cap could either get a proportionally longer default deadline or at least a pre-flight warning that the configured timeout looks tight for the payload. The existing `token-aware-diff-sizing` work unit in this repo is adjacent territory: that effort thinks about how much diff to send, this idea is about how long to wait for the answer, and whatever sizing signal that work produces would be the natural input for a scaled deadline.

Kept deliberately separate from the publish-step items logged the same day — this one is purely about the notes phase telling the user what it needs.
