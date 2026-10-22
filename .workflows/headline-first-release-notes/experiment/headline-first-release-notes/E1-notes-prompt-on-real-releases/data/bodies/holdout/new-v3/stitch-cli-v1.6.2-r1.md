✨ Added
- Choose the Claude model for AI features with `stitch config set ai.model sonnet`, or override it per project via `ai.model` in `.stitch/config.json` — leaving it unset keeps the Claude CLI default.

🔧 Changed
- The commit-msg hook's AI regeneration now uses the configured `ai.model`, read at runtime so existing installed hooks pick up changes without reinstalling.
