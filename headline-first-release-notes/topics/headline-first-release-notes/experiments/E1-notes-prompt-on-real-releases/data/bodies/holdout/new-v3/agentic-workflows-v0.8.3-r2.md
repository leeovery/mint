Starting a session no longer waits on the knowledge base embedding your documents.

✨ Added
- Vectors for new and changed documents are now filled in by a background task, so searches by meaning catch up moments after work finishes.
- A guide to running an embedding model on your own machine, covering which models held up in testing.
- `engine knowledge setup` now keeps your tuning settings (such as `similarity_threshold`) when you change provider.
- Discovery's topic preview now shows a short description of each proposed topic beside its summary.
- Migration summaries now render through the engine, and `engine commit --migrations` commits every path the workflows own.

💥 Breaking
- The knowledge CLI is gone — run `node .claude/skills/workflow-engine/scripts/engine.cjs knowledge <verb>` instead of `node .claude/skills/workflow-knowledge/scripts/knowledge.cjs`, and update any scripts or allowed-tools entries that call the old path.

🔧 Changed
- Knowledge base settings no longer stop anything: an unknown key or an out-of-range tuning value is ignored and named in search notes, `status` and start-up warnings.
- A failed vector fill is now recorded and reported in the next search, in `status` and in start-up warnings, and the next start retries it.
- Starting a session now commits migration changes that touch `.claude/settings.json`, `.worktreeinclude` and `.gitignore` together with the reviewed migration commit, instead of a separate commit.
- Implementation setup now asks about linters and project skills for a topic that never answered, and treats a confirmed empty answer as final.
- Questions at a gate now set it aside until you say you're ready to move on, and postpone and cancel requests are taken as said rather than checked back with a question.
- Roadmap items and parks must cite sources that already exist, and a park made before a session record exists carries no source.
- Discovery no longer records an open question or an unadopted suggestion as a settled decision.

🐛 Fixed
- Resuming a discovery session no longer opens a second session over the interrupted one.
- Cancelling or postponing a topic now writes up what the sitting discussed before the transaction.
- Resumed research and discussion sessions now say they are picking up where they left off instead of announcing a fresh start.
- Investigation write-ups no longer fill in symptoms nobody stated.
- Specification dependencies are rechecked and always written, and specification reviews no longer treat an analogy as the record's own answer.

🗑️ Removed
- `npm run build` and the bundled knowledge CLI; the knowledge base now installs as committed source with nothing to build.
