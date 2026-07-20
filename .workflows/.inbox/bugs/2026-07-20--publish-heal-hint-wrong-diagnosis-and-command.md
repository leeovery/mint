# Publish heal hint: wrong diagnosis, wrong command form

When the v0.6.0 publish step failed (2026-07-20), mint printed this hint alongside the failure:

```
⚠ publish failed  tag is already published; heal with regenerate --source tag
```

Both halves of the hint misled in this instance.

The diagnosis half — "tag is already published" — did not describe what happened. No GitHub release existed for the tag (`gh release view v0.6.0` returned "release not found" throughout); the publish had failed for an unrelated transient reason (API rate limiting, per the companion bug about swallowed stderr). The hint's wording asserts a specific cause with confidence, so the natural reading was that a duplicate release existed and needed reconciling — a wrong trail that cost investigation time before the real condition surfaced.

The command half — "heal with regenerate --source tag" — is not an invocation that exists. `mint regenerate` answers: `mint: unknown command (only mint release, mint release regenerate, mint init, mint version, mint commit, and mint setup are wired)`. The wired form is `mint release regenerate <version> [options]`, which also requires the version argument the hint doesn't mention, and in practice wants `--target release` for this scenario since its default target may rewrite more than the missing surface. A user pasting the hint verbatim gets an error; a user guessing the right subcommand still had to consult `--help` to assemble the working call.

Conditions: observed once, on a publish failure whose underlying cause was rate limiting; unclear whether the "already published" text is the sole hint for all publish failures or whether a detection branch misfired. Impact: during the one moment a release is half-out (tag pushed, page missing), the operator is handed a confident wrong explanation and a non-runnable recovery command — the two things a failure hint exists to prevent.
