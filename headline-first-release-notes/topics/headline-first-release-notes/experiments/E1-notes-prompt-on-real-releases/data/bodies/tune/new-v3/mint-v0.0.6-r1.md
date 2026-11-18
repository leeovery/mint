Let an AI agent configure mint for your project with the new `mint setup` command.

✨ Added
- `mint setup` prints a version-matched setup guide that an AI agent follows to inspect your project and propose a `.mint.toml` — it runs from any directory, no git repo needed.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` with an empty body and a short header pointing to the docs and `mint setup`, instead of a fully commented template of every key.
