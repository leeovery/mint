Starting a session no longer waits on the knowledge base's embedding step, because vectors now fill in the background.

✨ Added
- Knowledge vectors fill in the background — a finished document is searchable by keyword at once and gets its semantic vectors moments later.
- Knowledge problems are now reported rather than silent — an ignored config setting, a provider key that does not resolve, or a vector fill that fell short shows up in the next search note, `status`, and the start-of-session warnings.
- Local embedding models are documented as an unsupported but working route, with guidance on which models held up.
- `commit --migrations` records everything migrations changed in one reviewed commit, including removals.
- Discovery's topic proposal now shows a short description of each topic beside its summary.
- Migration summaries render through a dedicated display.
- Knowledge query failures show the failed query's error output alongside how to diagnose it.

🔧 Changed
- The knowledge base is now reached through `engine knowledge <verb>` — the separate `workflow-knowledge` skill and its bundled CLI are gone, and nothing needs building any more.
- Setup now changes only the provider settings in the config, so switching providers keeps your tuning values.
- Mistaken knowledge config settings are ignored and named instead of stopping searches.
- Questions asked at a gate now set it aside until you say you are ready, and the assistant asks in conversation before putting the gate back.
- Postpone and cancel requests are taken as said, and what the sitting discussed is written into the document first.
- Resumed discussion and research sessions open with a "picking back up" message instead of the fresh-start one.
- Discovery records only what the conversation settled as decisions; points you left open stay open questions.
- Agent file paths are given as installed from the project root rather than relative to the instruction file.

🐛 Fixed
- Unstarted implementations no longer record empty linter and project-skill lists as if you had confirmed them, so those setup steps ask when they should; a migration clears the stale empties.
- Roadmap items now refuse sources that do not exist yet, and a park no longer records a source before its session log exists.
- Resumed discovery sessions edit their existing log instead of opening a second one.
- Parking from a discovery session is now committed.
- A planning session's spec corrections are now reported before the revised designs are produced.
- The knowledge directory is untracked from git by a migration, so a stale tracked store no longer lingers.

🗑️ Removed
- The `npm run build` step and the bundled `knowledge.cjs` CLI.
