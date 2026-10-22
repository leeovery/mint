✨ Added
- Menus drawn as clickable buttons above the prompt in Claude Code's terminal app — click a row to pick it, click again (or press Enter) to send, and the buttons stay put while the transcript scrolls.
- Long menus page their rows under a fixed header and footer, so the buttons never scroll away.
- A held-send queue: pressing send while Claude is busy waits until Claude finishes, and only sends if the same menu is still showing.
- Menus come back when you resume a conversation, as long as nothing has happened since.
- Sent button answers show in the transcript as the question and the answer instead of Claude Code's plugin framing.
- First-run Claude Code setup step that switches the mod on and asks for a single restart, or says why it isn't running.
- Workflow sessions now get Claude's `SendUserMessage` tool and run without thinking summaries and the "say what you're doing" nudge, with your own settings restored afterward.
- Per-conversation folder beside the workflows' config (`~/.config/workflows/conversations/`) holding labels, buttons and answer rows, tidied away once Claude Code deletes the conversation.
- New engine-rendered gates for the dismissed topics list, defer gate, spec confirmation, plan context, cross-cutting specs, complexity check, first-phase choice, findings sign-off and completed-work actions.
- `npm run mod:types`, `typecheck:mod` and `test:mod` for working on the gate mods.

🔧 Changed
- Every menu now asks a glyphed question and has a key to press; hand-written menus in skill prose are gone in favour of engine-rendered ones.
- Pickers (archived inbox, working-set add/drop, manage list, completed work, experiment and baseline doc picks) are now numbered menus with a real back row.
- A reply that isn't a pick (a question, a comment) now sets a gate aside until you're ready to move on, and replies that change what happens next are confirmed first.
- Plain displays now use `text` code fences so they no longer draw in the menu colour.
- Every dispatch of a background agent states its mode, and waited-on dispatches end the turn on a single fixed sentence.
- Step 0 of `/workflow-start` is renumbered to fit the new Claude Code setup step before the walkthrough.
- Tmux label resume positions move into the conversation folder (migration 061) and are no longer pruned by age.
- Continue skills fetch their pick list through a new `select` call instead of the head-of-skill dump.
- Roadmap horizons are quoted in `roadmap add` commands so names with spaces work.

🐛 Fixed
- A menu fetched only for its data or display no longer gets drawn as a live gate.
- The walkthrough topic cards no longer carry a menu mid-conversation; it is fetched separately when needed.
