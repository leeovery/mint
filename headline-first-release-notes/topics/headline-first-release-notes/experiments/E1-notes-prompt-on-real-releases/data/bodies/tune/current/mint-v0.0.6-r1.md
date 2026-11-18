✨ Added
- `mint setup` command — prints a version-matched, AI-assisted setup guide so an AI agent can inspect your project and propose a `.mint.toml`.
- The setup guide includes a config reference table of every key, level, default and description, generated from a drift-tested source of truth.
- `mint setup` runs from any directory, with no git repository required.

🔧 Changed
- `mint init` now writes a minimal `.mint.toml` — an empty body with a short header pointing to the GitHub docs and `mint setup`.
- The unknown-command message and `mint help` now list `mint setup`.
- README gains a Setup section and a per-key config reference replacing the old commented template.
