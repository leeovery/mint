Menus now appear as clickable buttons above the prompt in Claude Code's terminal app, instead of being printed into the transcript.

✨ Added
- Clickable menu buttons above the prompt — click a row to put its answer in the prompt, click again (or press Enter) to send; typing still works.
- A Claude Code setup step on first `/workflow-start` — it switches the buttons on in the project's settings and asks for one restart, or stops with a note if Claude Code didn't pick them up.
- Gates survive a resume — a conversation resumed where nothing has happened since shows its buttons again.
- The gates waiting on you stay on screen while a background agent works, and an answer you send meanwhile is held, marked queued, and sent when Claude finishes.
- Workflow sessions turn off Claude Code's thinking summaries and its "say what you're doing" reminder, then restore your own settings afterwards.
- Sent answers read as the question and the answer in the transcript, e.g. `Approve this task? → yes · Commit and continue to next task`.

🔧 Changed
- The setup questions (walkthrough, session labels, knowledge base, baseline offer) now come after the Claude Code setup step, and the baseline offer appears on your first working session.
- Every menu now asks a question above its options, and the pick lists (inbox, archived items, working set, completed work, manage list, experiments, baseline docs) are real menus with a back row.
- Several prompts that were hand-written are now engine menus: spec confirmation, planning context, cross-cutting specifications, plan format, scoping complexity check, first-phase choice, findings sign-off, dismissed topics and the discussion's defer gate.
- A question asked at a gate now sets the gate aside until you say you're ready, then it comes back fresh; a reply that changes what happens next is confirmed first.
- Plain displays now render in the normal text colour instead of the menus' lilac.
- Agents the workflow waits on now end the turn with a one-line note such as "The executor agent has been dispatched for task 5.1."
- The continue skills fetch their pick lists fresh, with `v/view` for completed work.
- The missing-dependencies display is now drawn by the engine, and the roadmap add-to-delivery menu uses named keys.
- Conversation state, including tmux label positions, now lives in one folder per conversation beside the workflows' config and is removed once Claude Code deletes the conversation.

🐛 Fixed
- Resuming a conversation from another directory or checkout no longer loses its label position or buttons.
- Roadmap commands now quote horizon names, so names with spaces work.
- The specification handoff and reanalysis now route correctly to the specs menu when every grouping already maps to a spec.
