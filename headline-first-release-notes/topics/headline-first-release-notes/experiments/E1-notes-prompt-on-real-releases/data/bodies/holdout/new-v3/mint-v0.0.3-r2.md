Control which AI command and timeout each of `mint release` and `mint commit` uses, with a pinned default model.

💥 Breaking
- The default AI command is now `claude -p --model sonnet` instead of `claude -p` — set `ai_command = "claude -p"` in `.mint.toml` to keep using your Claude CLI's own default model.

✨ Added
- Per-verb `ai_command` override under `[release]` and `[commit]` — each falls back to the top-level value, then the default; `mint regenerate` uses `[release]`.
- New `timeout` setting (seconds) at the top level and per verb — raise it for slow AI commands, or set `0` for no limit.
- `mint init` template now includes the `timeout` key and commented per-verb `ai_command` and `timeout` overrides.

🔧 Changed
- A timeout is reported immediately as a failure and is never retried.
- A blank `ai_command` at any level now falls through to the next level instead of being used as-is.
