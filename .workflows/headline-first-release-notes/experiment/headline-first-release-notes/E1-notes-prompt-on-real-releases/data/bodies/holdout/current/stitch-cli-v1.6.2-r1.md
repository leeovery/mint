✨ Added
- Configurable AI model — set `stitch config set ai.model sonnet` globally, or `ai.model` in `.stitch/config.json` per project, to pin the model used for AI features.

🔧 Changed
- The commit-msg hook now honours the configured AI model at runtime — no reinstall needed after changing it.
- The `release` script is now a thin wrapper around `mint release` — requires `mint` to be installed (`brew install leeovery/tools/mint`).
