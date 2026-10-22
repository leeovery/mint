Control which AI command and time limit each of `mint release` and `mint commit` uses, with a pinned default model.

✨ Added
- Configurable AI timeout — set `timeout` (seconds) at the top level to change the per-attempt AI deadline, or `timeout = 0` for no limit.
- Per-command overrides — set `ai_command` and `timeout` under `[release]` or `[commit]` to change them for that command only, falling back to the shared value and then the default.
- `mint init` template now includes the `timeout` key and commented per-command `ai_command` and `timeout` overrides.

🔧 Changed
- Default AI command is now `claude -p --model sonnet` — zero-config runs use a fixed model instead of whatever your Claude CLI defaults to.
- Regenerating release notes uses the `[release]` AI command and timeout settings.
- A blank `ai_command` at any level now falls through to the next level instead of being used as-is.
