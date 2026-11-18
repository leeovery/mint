✨ Added
- Choose the Claude model for AI features with `stitch config set ai.model sonnet`, or override it per project via `ai.model` in `.stitch/config.json` — leaving it unset keeps the Claude CLI default.

🔧 Changed
- The generated commit-msg hook reads `ai.model` from your config at runtime, so model changes apply without reinstalling the hook.
