✨ Added
- Clickable menu buttons above the prompt in Claude Code's terminal app — click a row to pick it, click again (or press Enter) to send; typing still works everywhere.
- A second Claude Code mod that redraws a sent answer in the transcript as the question and the answer, instead of Claude Code's generic "plugin sent a message" framing.
- Workflow sessions now get Claude Code's `SendUserMessage` tool and run without thinking summaries or the "say what you're doing" nudge, so prescribed output reaches the screen reliably.
- A new Claude Code Setup step at `/workflow-start` — it switches the mod on in the project's settings and asks for a one-time restart, or tells you when Claude Code hasn't picked it up.
- Gate buttons survive a restart or `claude --resume`, returning to the gate you left as long as the conversation hasn't moved on.
- A per-conversation folder beside your workflows config, tidied automatically once Claude Code deletes the conversation.
- `npm run mod:types`, `typecheck:mod` and `test:mod` for working on the mods.

🔧 Changed
- Every menu now asks a question, and all remaining hand-written menus (spec confirmation, dismissed topics, complexity check, plan format, cross-cutting, and more) are engine-rendered.
- Picking from a list now uses numbered rows with a real back row across the inbox, manage, completed, working-set and baseline doc pickers.
- A question or comment at a gate now sets the gate aside until you're ready to move on, and replies that change what happens next are confirmed first.
- Plain displays now use `text` code fences so they no longer draw in the menu colour.
- Agents the workflow waits on now end the turn with a one-line "dispatched" sentence, and every dispatch states whether it runs in the background.
- Setup steps in `/workflow-start` are renumbered and the baseline and session-label prompts now carry their own step headings.
- The baseline assessment is now offered at your first working session, after the restart.
- Tmux resume positions move into the conversation folder.

🐛 Fixed
- Menus no longer appear for views that were fetched only for their data, such as continue-skill start-up dumps, the discussion defer gate and the roadmap read.
- The discussion close no longer asks to defer when the map has nothing on it.
- After specification regrouping, the entry now routes correctly when every grouping already maps to a specification.
