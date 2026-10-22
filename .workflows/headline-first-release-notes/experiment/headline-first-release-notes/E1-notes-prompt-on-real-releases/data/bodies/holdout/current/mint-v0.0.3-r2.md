✨ Added
- Configurable AI timeout — set `timeout` (seconds) at the top level to change the per-attempt AI deadline, or `0` to disable it.
- Per-verb overrides for `ai_command` and `timeout` under `[release]` and `[commit]` — pin a different model or deadline for one command without affecting the other.
- `mint init` template now scaffolds the `timeout` key and commented per-verb `ai_command`/`timeout` overrides.

🔧 Changed
- The default AI command is now `claude -p --model sonnet` — zero-config runs use a fixed model instead of whatever your Claude CLI defaults to.
- `ai_command` and `timeout` resolve independently as `[verb]` → shared → default, and `regenerate` uses the `[release]` settings.
- A blank `ai_command` at any level now falls through to the next level instead of being passed along empty.

🐛 Fixed
- Invalid values like `timeout = "fast"` or a non-string per-verb `ai_command` now fail at load with a clear error naming the key.
