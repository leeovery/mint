# Release notes: blind side-by-side
For each release, pick the notes that better meet the bar: good, readable, coherent, useful, accurate, well ordered. Reply with six picks, e.g. `1 A, 2 B, …`.

---

## 1. portal v0.11.1

*What the release was:* Fixes resume hooks that could silently vanish, so a restored pane came back to a bare shell; hooks saved in the old format stop firing after upgrade.

### A

Resume hooks now stay attached to a pane however tmux rearranges or renames it.

💥 Breaking
- Resume hooks are now keyed by a per-pane token instead of `<session>:<window>.<pane>` — existing hooks under the old keys no longer fire and are left in `hooks.json` untouched, so re-register them with `portal hook set` or remove them with `portal hook rm --pane-key <key>`.
- `portal hook rm` now exits non-zero when it removes nothing — scripts that relied on a silent success for a missing hook need to handle the error.

✨ Added
- `portal hook list` shows each hook's current pane location as a fourth column — empty when no live pane carries the hook.
- Session names starting with `$` or `-`, or containing `:`, are refused when renaming from the picker — the picker explains why and keeps the old name.
- The picker's notice band no longer collides with the theme panel — the panel lines up with the page beneath it when a message is showing.

🔧 Changed
- `portal doctor` explains why a stale-hook prune or check could not run (restore in progress, unreadable `hooks.json`, locked file, unreadable pane list) instead of reporting nothing.
- Generated session names never start with `-` or `$`, so a project named that way still gets a usable session.
- Stale-hook cleanup logs each removed hook's command at the default log level, so a reaped hook can be recovered.
- Saver daemon startup waits longer on slow machines before giving up, and bootstrap now reports a warning if it never comes up.
- Restored panes are armed with the exact `portal` binary doing the restore rather than whatever is found on `PATH`.

🐛 Fixed
- Concurrent `hook set`/`hook rm`/cleanup runs no longer overwrite each other's changes in `hooks.json`.
- Hooks registered while a stale-hook sweep is running are no longer deleted by it.
- Sessions with a period in their name, or a name that prefixes another live session, are no longer mistaken for missing sessions, killed, or dropped from saved state.
- Hooks survive session renames and tmux renumbering, and are restored after a reboot together with their pane.
- The filter footer keeps showing its keys on narrow terminals instead of vanishing.
- The `hook set` command asks the daemon to capture the new pane token right away.
- A corrupt `prefs.json` now reports which file is invalid.

### B

✨ Added
- Resume hooks now follow the pane itself — each hooked pane gets a durable token, so hooks survive pane moves and session renames.
- `portal hook list` shows a fourth column with each hook's current pane location (empty if no live pane carries it).
- Rename refusals in the picker — names containing `:` or starting with `$` or `-` are rejected with a clear message instead of leaving a session tmux can't address.
- `hooks.json` writes are now guarded by a lock file (`hooks.json.lock`) so concurrent writers can't overwrite each other.

🔧 Changed
- `portal hook rm` now exits non-zero when it removes nothing, so scripts can tell a real removal from a no-op.
- Stale-hook cleanup never deletes old-format `<session>:<window>.<pane>` keys it can't judge, and logs the command of every hook it reaps.
- `portal doctor` reports why a stale-hook check or prune couldn't run (restore in progress, unreadable `hooks.json`, locked file) instead of looking like it found nothing.
- Every per-session tmux command now uses an exact-match target, so a session whose name extends another's (or contains a period) is never confused with it.
- Generated session names no longer start with `$` or `-` or contain periods and colons from the project name.
- Restored panes are armed with the exact `portal` binary doing the restore rather than whatever is first on PATH.
- The saver-daemon readiness wait now tolerates slow starts and reports a warning if the daemon never comes up.
- Capture and shutdown flushes stand down when the restore marker can't be read, rather than risk saving a half-restored state.

🗑️ Removed
- The hidden `portal state migrate-rename` command — hooks no longer need re-keying on rename.

🐛 Fixed
- Hooks no longer get orphaned when a session is renamed or panes are rearranged.
- A hook registered while a cleanup is running can no longer be deleted by it.
- Live sessions with a period in their name are no longer treated as vanished and dropped from saved state.
- The filter footer keeps its keymap visible at narrow widths instead of disappearing.
- The theme panel stays aligned with the page when a notice band is showing.
- Config-migration failures now log the specific write-phase error class.
- Homebrew tap update runs as its own release job, so a tap failure no longer turns a published release red and can be retried alone.

---

## 2. tick v0.3.0

*What the release was:* Every TOON document rebuilt so a standard TOON parser reads it and free text survives read-edit-write; task descriptions change format (breaking for agent parsers).

### A

✨ Added
- `tick show --field` (alias `--fields`) selects which fields to print — one name returns the bare value, several return a trimmed document.
- Narrow list sections by position, e.g. `--field notes.2`, with real note indexes preserved.
- `--` end-of-flags marker — everything after it is treated as text, so titles and notes starting with a dash or spelling a global flag work.
- Flags now accept the attached form `--flag=value`, so values like `--` or `--json` can be passed literally.
- `create` and `update` output now includes a `changed` section listing tasks whose status moved (e.g. a done parent reopened).
- Notes in `show` output now carry a 1-based `index` matching what `note remove` accepts.

🔧 Changed
- `start`, `done`, `cancel` and `reopen` print one `changed` table (TOON) or list (JSON) with an `auto` flag marking cascaded tasks.
- TOON output is now valid, decodable TOON: single objects become named fields, tags/refs are inline lists, and descriptions are quoted single values.
- `stats` TOON output uses top-level fields instead of a `stats` row.
- `dep tree` always returns a populated document in TOON and JSON (empty graphs return zero counts), includes cycles and dangling blockers, and uses `trees` instead of `roots` in JSON.
- Focused `dep tree` documents open with the target's id, title and status and always include both `blocked_by` and `blocks` sections.
- TOON `dep tree` emits one edge per stored dependency, with summary values as top-level fields.
- JSON `dep add`/`dep remove` output uses a `blocker` key instead of `blocked_by`.
- `--ready` and `--blocked` now include in-progress tasks.
- `migrate` trims leading and trailing whitespace from imported titles and descriptions, and accepts `--from=beads`.
- `note add` accepts dash-leading text without needing the marker unless it spells a global flag.
- Help output aligns flag columns and documents the new `show` flags and the `--` marker.

🐛 Fixed
- Notes are listed in stored order rather than sorted by timestamp, so indexes match `note remove`.
- TOON output containing control characters it cannot encode now fails with an error naming the task and field, instead of printing a silently incorrect document.
- Pretty and JSON output no longer drop or misreport data for tasks hidden by cycles or missing blockers in `dep tree`.

### B

💥 Breaking
- TOON output shapes changed — `show`, `stats`, `dep tree`, `create`, `update` and status commands now print top-level `key: value` fields, `tags`/`refs` as inline lists, notes with an `index` column and descriptions as one quoted value, so scripts parsing the old layout must be updated.
- Status commands (`start`, `done`, `cancel`, `reopen`) now print a `changed` table in TOON and a `changed` list in JSON instead of arrow lines and the old `transition`/`cascaded` objects.
- JSON `dep tree` full-graph output renames `roots` to `trees`, and focused output always includes `blocked_by` and `blocks` and no longer carries `message`.
- JSON `dep add` and `dep remove` output renames the `blocked_by` key to `blocker`.

✨ Added
- `tick show <id> --field name,...` (alias `--fields`) selects fields, returns a single value bare, and narrows list sections by position such as `notes.2`.
- `--` ends flag parsing so dash-leading titles and note text can be passed, e.g. `tick create -- "--dry-run support"`.
- Flag values can be attached with `=`, e.g. `--title=--` or `--from=beads`.
- `create` and `update` output includes a `changed` section listing tasks whose status moved, such as a done parent reopened or an old parent auto-completed.
- Notes in `show` output carry their 1-based `index`, matching what `note remove` accepts.

🔧 Changed
- `dep tree` with no dependencies returns an emptied TOON/JSON document with zero counts instead of a prose message.
- `list --ready` and `--blocked` now consider in-progress tasks as well as open ones.
- Imported titles and descriptions from `migrate` are trimmed of edge whitespace.
- Help output aligns flag descriptions to the widest flag label per command.

🐛 Fixed
- TOON output is now properly quoted and round-trips awkward text (dashes, commas, quotes, blank lines) intact, and a value TOON can't encode fails with a clear error naming the task instead of printing a broken document.
- `dep tree` now shows tasks in cycles and dependencies on missing blockers rather than omitting them.
- Notes now display in stored order even when their timestamps run backwards.
- Empty task type in TOON lists is now quoted so rows decode correctly.

---

## 3. switchboard v0.2.6

*What the release was:* Performance: finished days stored as per-day digests so the sessions list, session pages and the Log's earlier days open fast; plus a small Runway tweak.

### A

Opening past days in the Log and session pages is faster, because finished days are now digested once and read back from saved results instead of re-reading every raw line.

✨ Added
- Finished days are now digested in the background, so session pages and the Log's past days load from saved results and fall back to the raw lines when none stand.

🔧 Changed
- The Log reads past days back through its filters, so reaching further back only reads the days that can hold matching requests.
- A session's turn details read only the days and stretches of the ledger that the turn spans.
- COMING UP on the Runway view no longer numbers accounts when the account column is collapsed — the numbers are gone from the lines and the help text.

🐛 Fixed
- The Log's find no longer applies a stale read of day summaries after it has been closed.
- The Log's Events no longer show days that were only partly read through a request filter.

### B

✨ Added
- Per-day digests of the request ledger — the router now digests each finished day in the background, storing session states so later reads resume from them instead of replaying every line.
- Per-day heads and row files for the Log — past days load their sessions and rows from stored summaries, so the Log's find and scroll-back open faster.
- Faster scroll-back for a session's turn — turns read only the days and row chunks that can hold their lines.

🔧 Changed
- The Log reads past days through the active filter, and re-reads days when the filter widens — fewer lines loaded while the Requests table stays consistent.
- Session pages rest on stored digest states and re-read a session whole only when those can't be used.
- Session page and turn totals are now priced when the page is built, so costs follow the prices of the day shown.
- The Log's find drops its day summaries when it closes and ignores reads that land afterwards.
- The Log's Events only show days that have been read in full, so a card never reports a day held through a filter.
- COMING UP no longer numbers accounts in the collapsed Runway view.

🐛 Fixed
- Request lines that the ledger files hold twice are now counted once, consistently across pages, turns, points and the Log.
- Reading a session's lines back from the end of a day's file no longer scans the whole file.

---

## 4. agentic-workflows v0.8.0

*What the release was:* A Claude Code mod draws the workflow gate menus as clickable buttons above the prompt.

### A

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

### B

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

---

## 5. agntc v0.5.2

*What the release was:* Packs' support folders, which 0.5.0 and 0.5.1 dropped, install again, with an update safety net; plus git retries and one-line errors.

### A

Packs can now ship shared folders alongside their skills, and updates no longer silently delete folders the repo still has.

✨ Added
- Pack support folders — folders in a pack's `skills/` with no skill in them (shared references, gates) now install whole to `.claude/skills/<name>/` beside its skills.
- Pack counts include support folders in the add menu, summaries and `list` detail, e.g. `2 skills, 3 folders, 1 agent`.
- `add --list` JSON output now reports each pack's support folders.
- Notes for a skill taken out of a pack now name the support folders left behind.

🔧 Changed
- A pack that holds two folders installing to the same place is now refused with "can't be installed as it stands" instead of offering a choice, and `update` aborts with no remedy.
- Update check failures print one line with the cause — `<repo>: couldn't check for updates (<cause>) — left untouched` — and `list` and version-change report failures the same way.
- Failed update lines no longer repeat the entry key, and copy-failure hints now name the cause.
- `list` detail counts each installed skill folder, hook and agent once, however many paths are recorded.
- A support folder landing on a loose skill from the same source is now reported as files in the way instead of being absorbed.

🐛 Fixed
- `update` now stops, naming the folders, instead of silently removing installed skill folders the repo still holds but would no longer install, and prints the `remove` + `add` that updates it anyway.
- Remote git reads (tag lists, update checks) now retry on transient failures, and a hung command times out with a clear message instead of failing on the first error.
- Cloning retries into a clean folder, so a partial clone no longer hides the real error, and an SSH "Permission denied" is retried rather than failing at once.
- Failed version fetches in `list` no longer crash the flow, and instead report why and leave the install untouched.

### B

✨ Added
- Pack support folders — folders in a pack's `skills/` that hold no skill now install whole to `.claude/skills/<name>/`, so skills can share references.
- Update safety check that stops and names any installed `.claude/skills/` folder an update would remove while the repo still has it, with a `remove` + `add` remedy to update anyway.
- Pack support folders appear in `add` menus, `--list` output (JSON included) and install summaries, e.g. `2 skills, 3 folders, 1 agent`.

🔧 Changed
- `list` detail view counts what an entry installs the way `add` does, telling skills from support folders.
- Skills taken from a pack with `--skill` now note the support folders left behind.
- Packs only absorb loose skills through their skills; a support folder landing on a loose skill is reported as files in the way.
- A pack whose own folders would install to the same place is refused with a clear message and no misleading "choose one" prompt.
- Failed update checks now print one readable line, e.g. `couldn't check for updates (<cause>) — left untouched`, and copy-failure hints name the cause.
- Update failure lines no longer repeat the member name.
- `list` detail view and version change report why a check or version fetch failed instead of failing silently.

🐛 Fixed
- Clones retry into a freshly emptied folder, so a partial checkout no longer hides the real error on retry.
- Remote reads (tags, default branch, update checks) now retry on transient failures, with per-attempt timeouts and clear "timed out" messages.
- SSH "Permission denied" errors are retried rather than treated as final, while HTTPS credential failures and timeouts fail immediately.

---

## 6. mint v0.0.2

*What the release was:* Two co-equal bodies: dry run becomes a true preview whose notes the real run reuses, and release output is redesigned around single-keypress gates.

### A

💥 Breaking
- The dry-run note cache moved from `.mint/cache` inside the repo to your user cache directory — previews cached by an earlier release are no longer found, so re-run `mint release -d` before relying on reuse.

✨ Added
- `mint release` asks you to confirm the version bump before any work begins — decline to abort with nothing touched.
- A real release that finds cached notes for the same diff offers to use them or regenerate.
- Dry runs are marked "dry run" from the first line and end with a "no changes made" summary instead of a success footer.
- Pressing Escape at a review gate declines it, and a second Ctrl-C force-quits if a prompt ever hangs.

🔧 Changed
- Review gates are single-keypress hotkey bars — press y/n/e/r with no Enter, and Ctrl-C aborts cleanly.
- Gate questions now say what accepting does, such as "Release with these notes?" and "Commit this message?".
- A dry run asks nothing: it previews the version, plan and notes, then stops without any review gates.
- Pretty output is redesigned flush-left, with notes and commit messages shown in a gutter panel and stages narrated as plain sentences.
- Slow steps show what they are doing, such as "generating commit message…", with a spinner for commit generation and regeneration.
- Release notes are now concise one-line bullets with no TL;DR summary.
- A cache miss is silent, with no "diff changed since dry-run preview" notice.
- The dry-run plan no longer lists the push step or checks the publish provider.

🐛 Fixed
- AI no longer narrates before commit messages and release notes — a closing reminder now ends each prompt.
- Failures no longer print twice when stdout and stderr share a terminal.
- When the AI fails during `mint commit`, a note explains why the editor is opening.

### B

✨ Added
- Single-keypress review gates — press y/n/e/r to decide with no Enter; Esc declines and Ctrl-C aborts cleanly.
- Version confirmation before a release starts — confirm the bump up front, so declining leaves nothing to undo.
- Cached-notes prompt on release — when notes for the same diff already exist, choose to reuse them or regenerate.
- Spinner and progress narration while commit messages are generated or regenerated.
- A `release` shim script at the repo root that runs `mint release` and points to the install command if mint is missing.

🔧 Changed
- `--dry-run` now previews the version, plan, and notes without prompting, then closes with a "no changes made" summary.
- Terminal output redesigned — flush-left layout, a gutter-style notes panel, a compact hotkey bar, and a verb-first header like `mint › releasing acme v1.4.0`.
- Review gates use clearer wording, such as "Release with these notes?" and "Commit this message?", with short action labels.
- Progress lines now read as plain sentences such as "Pushed branch + v1.4.0 atomically" and name what was recorded.
- Release notes are now one concise line per change, with no TL;DR.
- The note cache moved to your user cache directory, keeping project repos clean and pruning stale entries.
- AI prompts now end with a reminder to output only the message, curbing preamble and narration from the model.
- Commit now explains why the editor opens when AI generation fails.

🐛 Fixed
- A second Ctrl-C now force-quits instead of being swallowed by a stuck prompt.
- Failures no longer print twice when stdout and stderr share a terminal.
- Plain releases no longer show a misleading "diff changed since dry-run preview" notice when no preview existed.
