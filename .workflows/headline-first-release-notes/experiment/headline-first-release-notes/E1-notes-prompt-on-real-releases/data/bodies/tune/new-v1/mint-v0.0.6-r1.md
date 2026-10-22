Let an AI agent configure mint for your project with the new `mint setup` command.

✨ Added
- `mint setup` prints a version-matched guide an AI agent follows to inspect your project and propose a `.mint.toml` — it runs from any directory, with no git repo needed.
- The setup guide includes a full config reference listing every key, its level, default and meaning.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` — an empty body with a short header pointing to the docs and `mint setup`, since every default is built in.
- `mint help` lists the new `setup` command, and the unknown-command message now names it too.
