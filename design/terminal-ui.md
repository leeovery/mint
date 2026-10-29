# mint terminal UI design

mint's pretty output is a column of `✓ stage  detail` rows under a 🌿 header line, with a bulleted Plan, the notes behind a `│` gutter, a `[y] accept  [n] abort` bar and a 🌿 footer. agntc's redesign (its `design/terminal-ui.md`) showed what a deliberate look buys, so mint gets one too. Four looks were drawn on 28 Oct 2026 in the Paper file "Mint · terminal look" (sitrep sr-rwmhr1): Ledger, Tree, Rail and Receipt, each on the same six screens. Lee picked **Rail** on 29 Oct: the run drawn as a line from where it starts to where it lands. This doc settles that look and how it's built.

**Scope.** Only pretty output changes. Plain output (`--plain`, or stdout not a terminal) stays byte for byte, every line of it. The notes and the commit message ship exactly as written, as today. Exit codes, behaviour, the questions asked and the keys that answer them all stay. `mint setup`'s guide and the help texts stay. Wording stays, except where a sentence becomes a row and its subject moves into the row's name column.

**The frame.** Every line has a one-space margin. A run opens with one header line: ` mint ─ <verb>` on the left, `mint` bold in the accent and the `─` dim, and on the right, dim, `<where> · v<version>`, where being the repo root shortened to `~/…`. Short of room, the right side gives way the way agntc's does: where is cut from the left, then dropped, keeping the version. A dev build has no version. The layout is as wide as the terminal, between 60 and 100 columns, and 80 when the terminal gives none.

**The rail.** Column 1 is the rail. Every run starts at a station and ends at one, with nodes between them, one per stage:

| Mark | Colour | Meaning |
|---|---|---|
| `◇` | accent | the start station: where the run begins (`v1.3.2  main @ 7c1e0b4`) |
| `●` | accent | a stage done |
| `⠋` | accent | a stage running, braille at 80 ms a frame |
| `○` | dim | a stage still to come; the whole row is dim |
| `?` | accent | a question |
| `✗` | red | a stage failed; its name is red too |
| `!` | yellow | needs a look: a warning, a skipped version, a suggestion not asked about |
| `↩` | accent | the unwind |
| `◆` | accent | the end station: where the run landed (`v1.4.0  released`) |

A node's row is the mark, two spaces, the stage name bold in a 14-column name column, then the detail. A stage that takes time (a hook, an AI call, a push) has its time dim on the right, on the layout's last column. Lines that hang off a node (a hook's committed artifacts, a saved file, captured output) carry the rail's `│` in column 1 and start at the detail column. Between settled nodes the rail is a solid `│` in the accent wherever it shows; below the live node it's a dim `┆`.

**The plan is the rail.** Once the plan is known (after preflight), the stages still to come sit hollow below the live one, each with what it will do (`○ push  main + v1.4.0 → origin, atomic`), and the end station waits dim at the bottom. The bulleted Plan block goes. A stage that won't run (no `post_release` hook configured, a `--notes-file` release's check) isn't in the plan at all. The point of no return is drawn across the rail above the push: `┿━━ point of no return ━━…` to the layout's edge, dim while it's ahead; once the run crosses it, the `┿` takes the accent.

**The live region.** Everything from the live node down to the end station is redrawn in place: up the region's height, erase below, draw it again. The lines above it never change once printed. This is the one place mint moves the cursor up. It never uses the alternate screen and never clears the screen. A region line is cut to the width, never wrapped, so the region's height is always its line count; the width is read again at each redraw. The region redraws on every event and every spinner frame. Before mint hands the terminal to the editor, the region is erased (today's `SuspendSpinner`), and it's drawn again when the editor exits (`ResumeSpinner`). Warnings already go to the terminal once when stdout and stderr share it, so nothing writes under the region behind its back. The presenter contract changes to match: settled lines never change; only the live region at the bottom is redrawn.

**Progress.** A running stage's spinner sits in its mark's place, with its text dim in the detail column (`running the pre_tag hook…`). The notes of a release written in parts draw a bar: `part 3 of 4  ━━━━━━━━━━━━──────` in the accent and dim. A part re-cut as too big and the merge show as the bar's text (`part 2 was too big for the model, retrying it as 2 smaller parts`, `merging 4 parts`). The exclusion check runs as its own `check` node before `notes`.

**Questions.** A question is a `?` node: the question bold, then each key bold in the accent with its action after it (`y release   n abort   e edit   r regenerate`). Each key answers at a single keypress, as today. What the question is about hangs off the rail above it, the notes or the commit message indented four behind the `│`, and the plan still waits below it. On an answer the question and what hung above it erase, and the node it asked about settles with the answer as a dim tail: `● notes  written in 4 parts and merged · accepted`. An `e` or `r` redraws the body in place before the question comes back. Under `-y` nothing is asked and the tail reads `· accepted (-y)`.
- **The exclusion check's pause** asks `? docs/specs/**  Exclude it from the notes?` with the AI's reason, the files it matches and their size hanging above, and `e exclude   k keep`. Each answer settles as a line under the `check` node (`docs/specs/** excluded · design documents`); the answers' commit is a line of its own under them.
- **The notes floor** (`Release without AI notes?`, `c`, `e`, `a`) is a `?` node under the `✗ notes` node and its output.
- **The cached-notes question, and regenerate's `Source?` and `Target?`,** are `?` nodes the same way. Source and Target read a typed line, with the cursor on the node.
- **A dry run's preview** of a question shows the `?` node with its question and no keys.

**Failure and the unwind.** A failed stage is a `✗` node with its message. The command's own output follows verbatim behind the node's rail, now red, and a dim `│ ` gutter of its own, so it never reads as mint's words. A saved part-notes path gets a line of its own. A warning is a `!` node. The unwind is a `↩` node naming what it undid, and the rail then returns to the start station: `◇ v1.3.2  back where it started · repo clean · nothing pushed`, with the run's time on the right. An unwind that couldn't finish is a `✗ unwind` node with the manual step. Past the point of no return nothing unwinds: a failure there is a `!` node, the end station still reads `◆ v1.4.0  released`, and the exit code is today's (a failed publish or `post_release` warns and exits 0). A run that stops before it changes anything ends at `◇ <start>  nothing changed`.

**Next.** Remedies don't interrupt the rail. A row says what happened; what the reader can do about it prints after the end station under a bold accent `next`. Each item is a lead line, then its command or config line indented five, in the accent, alone on its line: never cut, never wrapped, never in a box, so a triple-click copies it clean. The notes failure's next moves come first, then the exclusion check's suggestions, each with the `diff_exclude` line that acts on it.

**The close.** The end station is the close: `◆ v1.4.0  released` and the run's time, with the release's URL on the line below in the accent. Commit lands at `◆ main  a3f9c21 · pushed`, and regenerate `--all` at `◆ done  6 of 7 versions regenerated · 1 skipped`. A dry run ends at `◇ v1.4.0  dry run · nothing changed`, the stages it skipped left hollow above it.

**Colour.** Statuses use the terminal's own colours, so the theme decides how they look. The one brand colour is the accent, mint `#5FD7AF`: in truecolor where `COLORTERM` says so, as xterm-256 colour 79 (the same colour exactly) where `TERM` says 256, and the theme's cyan otherwise. Names are bold, metadata and times dim, rules dim. `NO_COLOR` turns colour off. Meaning never rests on colour alone: every state has its own mark. The 🌿 leaves the output, its place taken by the `mint` wordmark; `commit_prefix`'s 🌿 in the release commit's subject is that commit's text and stays.

**Width.** Settled lines wrap a long detail inside the detail column; the live region cuts instead. Below 70 columns the times drop and a detail goes on the line under its name.

**Per command:**
- **release:** start station `◇ v1.3.2  main @ <sha>`. Nodes: version (`v1.4.0 (minor bump)`, today's wording), preflight, `pre_tag` with its artifacts commit's subject hanging under it, check, notes, record, the point of no return, push, publish, `post_release`. End station `◆ v1.4.0  released`, URL under it.
- **release regenerate `<version>`:** start station `◇ v1.4.0  <source> → <target>`. Nodes: the notes (from the AI, the tag, the release or the file), the review, then `CHANGELOG.md` (rewritten, committed, pushed) and the GitHub release, as the target has them. End station `◆ v1.4.0  regenerated`.
- **release regenerate `--all`:** start station `◇ 7 versions  fresh notes → GitHub releases, then CHANGELOG.md`. A node per version, oldest first; a skipped version is a `!` node with its cause, and its retry command goes under next. Then the `CHANGELOG.md` node (rebuilt, committed, pushed once) and the end station. Today's one block per version, each with its own header, goes.
- **commit:** start station `◇ main  7c1e0b4`. Nodes: the diff (`4 files · +86 −12`, and what `-a`/`-A` will stage), the message hanging off the rail, `? Commit this message?`, stage, commit, push. End station `◆ main  a3f9c21`, with `· pushed` after a push (`-p` pushes to the upstream, which may not be origin). A failed `-p` push is a `!` node with git's stderr verbatim; the end station still names the commit, and the run exits 1 as today.
- **init:** no stations; a `●` row per file created and a dim `○` row per file left alone, its reason dim.
- **version:** ` mint v0.1.2` with `mint` in the accent. Plain stays the bare value.

**The Presenter seam.** `Presenter` stays the one output surface and keeps its events. Its payloads carry what the rail draws, so the presenter still renders only what it's handed:
- **`RunStarted`** carries the header (the command words, where, mint's version) and the start station: where the run begins and its detail. One read-only git call before it gives the branch and the short HEAD.
- **`ShowPlan`** carries the run's stages in order, each with a key, its name and what it will do, and marks the point of no return. The verb/target bullet list goes.
- **Stage events** carry their stage's key, so the presenter fills the right node. A failed stage carries its time.
- **Questions** carry the key of the node they ask about; the exclusion pause carries its suggestion whole.
- **`Unwound`** carries what was undone, what was kept and where the repo ended up as separate fields, not one sentence.
- **`NextMoves`** items become a lead plus a command line, as agntc's remedies are. A move whose flag or config sits mid-sentence stays a lead alone, its wording as today's. Plain flattens them back to today's sentences byte for byte.
- **`RunFinished`** closes every run that `RunStarted` opened, exactly once: how it ended, the end station and the run's time. Plain still writes its line only for today's successful endings.
- **`Quiet`** marks an event that only fills in the run's structure (release's early plan, a node no event settled before): plain's log, pinned byte for byte from before the rail, writes nothing for it.
- **The `Leaf` fields go,** with the 🌿.

**Testing:**
- **The renderer** is pure functions from rows, nodes and stations to lines, tested at widths 60, 80 and 100 with colour off, plus exact escape codes for each element with colour on.
- **The live region** is tested by its exact writes: the cursor-up count, the erase, the redraw, and the erase and redraw around the editor.
- **Golden runs** render whole runs at width 100 with colour off and compare against expected text: a release through its review, a release that fails and unwinds, a dry run, a commit with a push, and regenerate `--all` with a skip.
- **Golden frames,** as Switchboard's: each golden run is also kept as `testdata/frames/<name>.txt` and `.ansi`, rendered in process from fixed payloads, so times and hashes never drift. A test parses the ANSI into cells and compares glyph, foreground, background and bold, listing the first 20 cells that differ. An escape it doesn't know fails the test rather than being guessed. `go test ./internal/presenter -run Frames -update` writes them again. Switchboard's `internal/capture/reference_test.go`, `cells_test.go` and `frames.go` are the model.
- **Plain** keeps its goldens unchanged, and they must pass untouched.

**Pictures.** VHS records the README's art the way agntc's `demo/` does, which is itself Switchboard's: real commands typed into a real terminal, against a world built for them. The same recordings are how a build slice checks its own work: record the verb's tapes, look at the frames, and fix what reads wrong before review.
- **The world** (`demo/world.sh`, built under the gitignored `demo/out/world/`) has `pigeon` at `~/Code/pigeon` in its own `HOME`, with a bare origin that git reaches as `https://github.com/acme/pigeon` through an `insteadOf` in the world's own gitconfig (`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_NOSYSTEM=1`). Its commits and tags are dated, so their hashes are the same in every recording. `TZ=UTC`.
- **Shims on `PATH`:**
  - `mint` is built from the checkout with a pinned version through `internal/buildinfo`, so the header shows a release.
  - The AI command, set as the world's `ai_command`, answers from canned replies after a short sleep, so spinners show. It picks a reply from what the prompt asks: the exclusion check, a part, the merge or a commit message. An environment switch makes part 3 fail with a usage-limit message, for the failure tape.
  - `gh` answers `auth status` and the release calls.
  - `git` sleeps briefly on `push`.
- **Tapes** share one `demo/look.tape` through `Source`: Lee's Ghostty (JetBrains Mono, Nordfox, `LetterSpacing 0`, `LineHeight 1`, `CursorBlink false`), 100 columns, the shell prompt as `WaitPattern`. Each tape builds the world hidden, then types its command. It waits on output with `Wait` or `Wait+Screen /…/`, and sleeps only to pace a clip. The tapes:
  - `tour`, a clip: a release through the exclusion pause and the review to the end station
  - `failed`: `release -m -y` stopping on the notes and unwinding
  - `dry-run`
  - `commit`: `commit -Ap`
  - `regenerate`: `regenerate --all` with a skip
  - `init`
- **Recording:** `demo/record.sh <name>` deletes the tape's old output, runs `vhs` with `TMPDIR` inside `demo/out/` (vhs writes nothing, and says nothing, when `TMPDIR` is on another volume), then `demo/finalize.sh`. That lays the cursor frames over the text frames, adds a margin and writes a lossless WebP: the last frame for a still, every frame for a clip, plus an MP4. Art is rendered at twice its display width and committed to `art/`. `brew install vhs ffmpeg webp`; vhs 0.12 or later.
- **Gotchas** go in `demo/README.md`, as Switchboard's `testdata/vhs/README.md` has them:
  - vhs's browser reaches ttyd over loopback, which an agent's sandbox blocks.
  - A `Set` after any other command is ignored.
  - A path with `/` in it must be quoted.
  - vhs can fail without a word and leave an old frame behind, so `record.sh` deletes the output first and checks that frames exist.
- **The README** leads with the `tour` clip at 840 px, and each command's section gets its still.

**Out of scope:** an ASCII fallback for terminals without Unicode, colour settings, a `--no-color` flag (`NO_COLOR` covers it), and rewording beyond what the rows need.

## Known limits

- **A terminal narrowed mid-run** below a live line's width wraps that line once before the next redraw reads the new width, and leaves one stray line above the region.

## Build

Built without the `.workflows` pipeline, from this doc, as a stack of four PRs. Each is reviewed at `/code-review high` as it's ready, and the whole change at `xhigh` before it's marked ready:

1. **The payloads.** The engine supplies the start and end stations, the plan's stages with their keys and the point of no return, stage keys, the unwind's fields and the next moves as lead and commands. Pretty still draws today's look from them, and plain stays byte for byte.
2. **The demo world and tapes,** recording today's look. This runs beside 1, so every later slice has the world to check itself against.
3. **The rail renderer and the live region,** built under `internal/presenter` and not yet wired in: colour levels and the accent, width, the rows, nodes, stations, hanging lines, questions, next and the close, the region writer, and the golden-frame harness.
4. **The switch,** one slice per command: release, then regenerate, then commit, init and version.
   - Each slice moves its verb's pretty output onto the rail, writes its golden frames, and records its tapes, which are checked by eye before review.
   - The last slice deletes today's pretty code and the `Leaf` fields, brings CLAUDE.md's presenter rules up to the live region, records the art and puts it in the README.
