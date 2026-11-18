Let an AI agent configure mint for your project by running `mint setup` and following what it prints.

✨ Added
- `mint setup` prints a version-matched AI setup guide covering mint's pipeline and hooks, with a full config-reference table — runs from any directory, no git repo needed.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` with an empty body and a short header pointing to the docs and `mint setup`, instead of a fully commented template of every key.
- The unknown-command error now lists `mint setup` among the available commands.
