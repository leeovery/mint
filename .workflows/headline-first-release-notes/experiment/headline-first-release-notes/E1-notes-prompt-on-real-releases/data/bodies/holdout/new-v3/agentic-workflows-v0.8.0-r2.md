Menus now appear as clickable buttons above the prompt in Claude Code's terminal app, instead of being printed into the transcript.

✨ Added
- Clickable menu buttons — click a row to put its answer in the prompt, click again (or press Enter) to send; typing still works, and long menus page instead of overflowing.
- A button that stays on screen while background work runs — a second click is held as "queued" and sent when Claude finishes, only if the same menu is still showing.
- Menus come back when you resume a conversation that was left at one.
- Sent answers show in the transcript as the question and your answer, such as `Approve this task? → yes`.
- A one-time setup stop in `/workflow-start` — it switches on the Claude Code function-hooks flag in the project's settings and asks for one restart, or tells you the buttons aren't running.
- Workflow sessions turn off Claude's thinking summaries and its "say what you're doing" reminder, so shown content reaches the screen as written, and put your own settings back when the conversation ends.
- A Dismissed Topics list in discovery that offers re-add actions, and an empty-state line when nothing is dismissed.

🔧 Changed
- Every menu is now drawn by the engine and always asks a question; the last hand-written menus in the prompts are gone.
- Menus that used to print into the transcript — the selection lists, dismissed topics, the inbox working set, planning, scoping, and specification confirmations — now show as pickable rows.
- The specification confirmation shows what the handoff will do (sources, consult references, paths) above a single Proceed? gate.
- Plain displays now render in the normal text colour instead of the menu's lilac.
- A question asked at a gate sets the gate aside until you say you're ready to move on, and a reply that changes the plan is confirmed before it is acted on.
- Background agents now end their turn on a one-line "dispatched" sentence.
- The startup steps are renumbered, with the baseline offer and session-label prompt coming after the new setup stop.
- tmux session-label positions and the buttons' state now live in a per-conversation folder beside the workflows config, cleaned up once Claude Code deletes the conversation.

🐛 Fixed
- Resuming a conversation from another directory now brings back its tmux label and buttons.
- Menus that were fetched only for their data no longer show up as live gates.
- The completed/cancelled and archived lists, and the manage list, no longer print a numbered list above a menu that re-lists it.
