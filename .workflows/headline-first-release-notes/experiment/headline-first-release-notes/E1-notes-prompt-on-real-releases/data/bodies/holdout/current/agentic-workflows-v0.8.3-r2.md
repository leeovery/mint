✨ Added
- Background vector fill — a start or phase completion saves keyword search immediately and gives chunks their meaning-search vectors moments later, so nothing waits on the embedding service.
- `engine knowledge` is now the single door to the knowledge base (`index`, `query`, `check`, `status`, `remove`, `compact`, `rebuild`, `fill`, `setup`), replacing the separate bundled CLI.
- Local embedding models are documented as an unsupported-but-open route, with guidance on which models held up in testing.
- `commit --migrations` records everything a migration changed, including files it untracked, in one reviewed commit.
- A mistaken knowledge setting (unknown key or out-of-range tuning value) is ignored and named in search notes, `status` and start-up warnings instead of breaking anything.
- Discovery topic proposals now carry a short description of what each topic covers and why it surfaced.

🔧 Changed
- Starting a session no longer waits on embedding — boot brings keyword search in line with your files and hands vectors to the background fill.
- Changing embedding provider in setup keeps your tuning overrides and every other key in the config file.
- Failed vector fills are reported in the next search note, `status` and start-up warnings, and retried at the next start.
- The knowledge base no longer needs a build step — it ships as plain engine modules.
- Gates set aside by a question are now put back only after asking in conversation whether you're ready to move on.
- Cancelling or postponing a topic records what the sitting discussed first, and takes the request as said without asking again.
- Discovery briefs and topic summaries keep undecided points as open questions rather than promoting them to decisions.
- Resumed discussion and research sessions open with a "picking back up" message instead of the first-start intro.
- Implementation setup now asks about linters and project skills until you confirm, even if the answer is none.
- The knowledge query-failure gate now shows the failed query's error output and how to diagnose it.
- The migration summary is now rendered by the engine.
- Agent and reference paths handed to agents are written as installed paths from the project root.
- Roadmap sources must exist on disk and be written relative to `.workflows/`.

🐛 Fixed
- Discovery no longer opens a second session log over an already-installed one when resuming or after epic creation.
- Parking an idea on the roadmap before a session record exists no longer fails on a missing source.
- Migrations that remove tracked files from git are now committed correctly instead of being silently left behind.

🗑️ Removed
- The `workflow-knowledge` skill and its bundled CLI, the esbuild build step, and `npm run build`.
