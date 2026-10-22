✨ Added
- Configurable AI model — set `stitch config set ai.model sonnet` globally, or `ai.model` in `.stitch/config.json` per project, to choose which Claude model powers AI features.

🔧 Changed
- The commit-msg hook now reads the configured AI model at runtime — changes to `ai.model` apply without reinstalling the hook.
- The `release` script is now a thin wrapper that delegates to `mint release` — requires installing mint via `brew install leeovery/tools/mint`.
