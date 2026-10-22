✨ Added
- Vectors now fill in the background — an index or phase completion returns at once, searchable by keyword immediately, with semantic vectors landing moments later.
- `engine knowledge fill` embeds any chunks still awaiting vectors, one fill at a time.
- Local embedding models are documented as an unsupported-but-possible route, with guidance on which models held up and what to look for.
- Discovery's topic proposal now shows each topic with a short description of what it covers and why it surfaced.
- `engine commit --migrations` commits everything the workflows own, including removals a migration staged.
- Migration 062 clears empty `project_skills` and `linters` arrays from implementation topics that were never asked.
- Migration 063 retires the old knowledge store file and its `.worktreeinclude` line.
- Migration 064 untracks the knowledge directory from git, leaving the files on disk.

🔧 Changed
- The knowledge base is now part of the engine — every call goes through `engine knowledge <verb>`, replacing the separate `workflow-knowledge` skill and its bundled CLI, with nothing to build.
- Start-up never waits on embedding — boot brings the keyword index in line and hands vectors to the background fill.
- A failed vector fill is reported in the next query's note, in `status`, and in boot's warnings, and is retried at the next start.
- Setup now replaces only the provider settings in a config, so changing provider keeps your tuning values.
- Mistaken knowledge config keys or out-of-range tuning values are ignored and named instead of stopping the knowledge base.
- The query-failure gate now shows the failed query's error and the `knowledge status` command to diagnose it.
- Migration summaries at start-up now render through the engine.
- Agent and reference paths in skill prose are written as installed (`.claude/...`) rather than relative to the file.
- Questions asked at a gate now set it aside until you say you're ready to move on, and the assistant asks before putting it back.
- Postponing or cancelling a topic is taken as said, and unrecorded discussion is written into the document before the transition.
- Resumed discussion and research sessions open with a "picking back up" line instead of the fresh-start greeting.

🐛 Fixed
- Discovery no longer opens a second session over a resumed or just-created epic's log.
- Discovery briefs and topic descriptions no longer turn a point the user left open into a settled decision.
- Roadmap items can no longer record a source session log that doesn't exist yet, and sources prefixed with `.workflows/` are refused.
- Parking an idea before the session has a record no longer fails on a missing source.
- Implementation setup now asks about linters and project skills when never asked, instead of treating an empty list as an answer.
- Planning reports spec corrections once per pass, and specification dependencies are re-checked rather than rewritten.

🗑️ Removed
- The `workflow-knowledge` skill, its bundled `knowledge.cjs` CLI and the esbuild build step.
