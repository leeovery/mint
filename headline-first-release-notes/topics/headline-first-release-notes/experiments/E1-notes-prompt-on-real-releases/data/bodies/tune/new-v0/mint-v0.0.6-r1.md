Let an AI agent configure mint for your project with the new `mint setup` command.

✨ Added
- `mint setup` prints a version-matched setup guide for an AI agent to follow — it inspects your project, proposes a config, and includes a reference of every config key with its level and default.
- `mint setup` runs from any directory, with no git repository required.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` with an empty body and a short header pointing to the docs and `mint setup` — every default is built in, so an empty file is valid.
- The unknown-command error now lists `mint setup` among the available commands.
