Let an AI agent configure mint for your project with the new `mint setup` command.

✨ Added
- `mint setup` prints a version-matched setup guide for an AI agent — it inspects your project, proposes a minimal `.mint.toml`, and includes a full config reference of every key, level and default.
- `mint setup` runs from any directory and needs no git repository.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` with an empty body and a short header pointing to the docs and `mint setup` — the file is optional since every default is built in.
- The unknown-command message and `mint help` now list `mint setup`.
