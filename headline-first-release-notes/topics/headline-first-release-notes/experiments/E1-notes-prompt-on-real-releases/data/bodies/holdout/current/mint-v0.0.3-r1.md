✨ Added
- Configurable AI deadline — set `timeout` (seconds) at the top level to change the per-attempt AI limit, with `0` meaning no limit.
- Per-verb overrides — set `ai_command` and `timeout` under `[release]` or `[commit]` to use a different AI command or deadline for just that verb.
- `mint init` template now scaffolds the shared `timeout` and commented per-verb `ai_command`/`timeout` overrides.

🔧 Changed
- Default AI command is now `claude -p --model sonnet` — zero-config runs use a fixed model instead of whatever your Claude CLI defaults to.
- `ai_command` and `timeout` resolve independently as `[verb]` → shared → shipped default, and `regenerate` uses the `[release]` settings.
- A blank `ai_command` at any level now falls through to the next level instead of being used as-is.
- Negative `timeout` values are ignored and fall through to the next level.
- AI timeouts are fatal and never retried — only bad output is retried once.
