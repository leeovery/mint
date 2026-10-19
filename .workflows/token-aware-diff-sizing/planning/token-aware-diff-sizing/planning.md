# Plan: Token Aware Diff Sizing

## Phases

### Phase 1: Lean default AI command and quote-aware parsing
status: draft

**Goal**: Every AI call mint makes runs through the lean shipped default command. Every `ai_command` is split into a program and its arguments by POSIX quoting rules. A malformed one is rejected when config loads, while a blank one still counts as unset. Release-notes calls get their own 180-second shipped timeout. A failed `mint commit` generation shows the AI's own output verbatim.

**Why this order**: Every call the later phases add uses this one command (§2.2): chunk, merge, classify and the exclusion check. Its empty `--tools ""` argument and its multi-word system prompt reach claude only through quote-aware parsing (§3), so the transport foundation comes first. The 180s release timeout lands before calls grow to the cap in Phase 2, so a call at the cap finishes inside it (§4.4). Commit's verbatim failure output lands with the command because a claude too old for the new flags has to say so on commit (§2.5, §17).

**Acceptance**:
- [ ] Given a repo that sets no `ai_command`, when `mint release` writes notes or `mint commit` writes a message, the program run is `claude` with the arguments `-p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."`. `--tools` is followed by one empty argument, the system prompt arrives as one argument, and no shell is involved
- [ ] Given an `ai_command` at any level that uses quoting, such as `tool --system-prompt="Be terse" '$HOME' a\ b ""`, the tool receives `--system-prompt=Be terse`, the literal `$HOME`, `a b` and one empty argument, with no variable expansion, globbing, pipes or redirection
- [ ] Given an explicit `ai_command` with no quotes or backslashes at any level, the program and arguments run are the same as before the change, and the lean default never replaces it
- [ ] Given a `.mint.toml` with an unterminated quote in `[release].ai_command`, a shared `ai_command` ending in a lone backslash, or a `[commit].ai_command` of `'' --flag`, whose first word is empty, any command that loads config fails before any step runs with, respectively, `invalid .mint.toml: release.ai_command has an unterminated quote`, `invalid .mint.toml: ai_command ends in a lone backslash` and `invalid .mint.toml: commit.ai_command names no program`. A non-blank value whose split yields no words at all also fails as `names no program`
- [ ] Given a blank or whitespace-only `ai_command` at any level, config loads and that level counts as unset: resolution falls through to the next level as built. A blank `[commit].ai_command` under a shared `ai_command` runs the shared command, and with nothing else set the lean default runs
- [ ] Given no timeout set at any level, a release-notes call (forward release or regenerate) that hangs fails as `AI timed out` after 180 seconds, and a `mint commit` call after 60 seconds. An explicit `[release].timeout` or shared `timeout` still wins over the 180s
- [ ] Given an AI tool that exits non-zero with a message, such as a claude that rejects `--safe-mode` as an unknown option:
  - an attended `mint commit` prints `could not generate a message, opening editor` with the tool's captured output verbatim beneath it, then opens the editor
  - an unattended run fails with `no AI message and no interactive editor available`, with that output verbatim beneath it
  - `--no-ai` and an all-excluded diff still reach the editor with their lines unchanged and no AI output
- [ ] The README documents:
  - the lean default command in full
  - what it drops on purpose: CLAUDE.md files and Claude Code's own system prompt
  - that conventions belong in the `[release]` and `[commit]` `context` and `prompt` keys
  - `ai_command`'s quoting rules
- [ ] `mint setup`'s AI-transport paragraph names the lean default and the 180s release and 60s commit timeouts, in place of `claude -p --model sonnet` and `default 60`
- [ ] The README tables and `mint setup`'s config reference show `[release].timeout` defaulting to 180s, and the shared `timeout` row names both shipped defaults

### Phase 2: Per-call size cap and chunked release notes
status: draft

**Goal**: `max_prompt_bytes` replaces `max_diff_lines` as a byte cap on every composed prompt. Release notes whose single-pass prompt would exceed the cap are written by parallel chunk calls and one merge call, for forward releases and regenerate's fresh source alike.

**Why this order**: This is the feature's core: AI-written notes for a release of any size (§1.1). It runs on Phase 1's lean command, which the 1.5 MB default is calibrated against (§4.5). Every later phase reacts to or refines the calls this phase makes.

**Acceptance**:
- [ ] Given no `max_prompt_bytes` set, a release whose composed single-pass prompt (instructions, Change Map, diff and closing reminder) is at most 1,500,000 bytes makes one AI call as before. A release over that size makes no single-pass call and is chunked
- [ ] Given an over-cap release, the AI tool receives chunk calls, never more than four at once and none bigger than the cap. Each carries:
  - the release's instructions
  - the whole-release Change Map
  - framing that names its part (part k of n)
  - its slice of files, in path order
  - the closing reminder
- [ ] Given those chunk calls succeed, exactly one merge call follows, carrying every chunk's notes in chunk order, and its output ships as the notes body byte-identical
- [ ] Given a file whose diff alone exceeds a chunk call's room, including a single minified line, it is sent as piece calls cut at hunk boundaries, else line boundaries, else character boundaries. Each piece carries the file's diff header and says which piece of how many it is, and nothing in the diff is dropped
- [ ] Given a release whose Change Map would take more than a tenth of the cap, such as one deleting 4,312 files under `vendor/`, every call that carries the map receives it condensed by folder (`Removed: 4,312 files under vendor/`), single-pass included. The largest runs collapse first, and the same input condenses identically on every run. A map under its share is sent exactly as before
- [ ] Given an outsized `[release].prompt` or `context` that leaves no room for diff even with the Change Map condensed, no AI call is sent and the notes fail as `no room for the diff under max_prompt_bytes`. Given chunk notes whose merge prompt would exceed the cap, the merge is not sent and the notes fail as `part notes too large to merge`
- [ ] Given a chunk call that fails, no merge runs and no further chunk calls start (calls already in flight finish). The notes then fail through `on_notes_failure`, as a single-pass failure does
- [ ] Given a chunked run in pretty mode, the one notes line updates as `Writing notes: {size} release, in {n} parts · {k} of {n} done`, then `Writing notes: merging {n} parts`, and ends `✓ Notes written ({n} parts, merged)`. In plain mode each update prints as its own line, including one per finished part. A single-pass run narrates as before
- [ ] Given `regenerate <version>` from a fresh source, or `regenerate --all`, on a version over the cap, that version's notes are chunked and merged the same way, and no version is skipped or failed as `diff too large`
- [ ] Given a `mint commit` whose composed prompt exceeds the cap, no AI call is made and the editor opens with the existing `diff too large to summarise — opening editor` note
- [ ] Config load enforces the new key:
  - a `max_prompt_bytes` of zero or less fails before any step with `invalid .mint.toml: max_prompt_bytes must be a positive number of bytes`
  - a non-integer value gets a friendly type error
  - a `.mint.toml` still setting `max_diff_lines` fails with `invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes`
  - `max_prompt_bytes` under `[release]` or `[commit]` is rejected as an unknown key
- [ ] Docs:
  - The README's per-key tables and `mint setup`'s config reference show `max_prompt_bytes`, defaulting to 1,500,000, and no `max_diff_lines`
  - The setup guide's AI-transport paragraph covers the cap and chunking
  - The README documents chunking with `mint release`
  - CLAUDE.md's config seam names `max_prompt_bytes` as shared-only

### Phase 3: `--notes-file` and one non-AI body
status: draft

**Goal**: An operator or agent can release, or put new notes on one already-released version, with a finished body read from a file that mint keeps out of git. On an attended run the body shows first at that command's own review gate. `[release].fallback` is removed, and every route that ends without AI notes gives the release's commit-subject list.

**Why this order**: It stands apart from the AI ladder, and later phases rely on what it builds. Phase 4's saved chunk notes are released through `--notes-file`. Phase 5's floor offers the commit-subject list as `c` and points at `--notes-file` in its guidance. It also sends a regenerated version's `c` and `e` bodies to the regenerate review gate without `r` that this phase adds for a supplied body. The removed-key message for `[release].fallback` names `--notes-file`, so the two land together.

**Acceptance**:
- [ ] Given a clean tree and an untracked `notes.md` at the repo root, `mint release -y --notes-file notes.md` passes the clean-tree check, makes no AI call, and ships the file's contents byte-identical as the tag message, changelog entry and provider release. Afterwards `notes.md` sits untouched and untracked in the work tree
- [ ] Given that file with `--autostash` and other uncommitted changes, the stash leaves `notes.md` in place. Given a pre-tag hook that changes files, the pre-tag artifact commit does not contain `notes.md`
- [ ] Given `--notes-file` on a first release, or on a release whose post-exclusion diff is empty, the supplied body ships instead of `Initial release.` or the maintenance-release body
- [ ] Given `--notes-file` with `--no-ai`, mint exits 2 with a usage error. Given an empty or whitespace-only file, it exits 1 before any step with `--notes-file P is empty`. Given a missing or unreadable file, it exits 1 before any step with `reading --notes-file P: <cause>`
- [ ] Given a released version, `mint release regenerate <version> --notes-file notes.md` puts the file's body on that version with no AI call. Combined with `--all` or `--source`, it exits 2 with a usage error
- [ ] Given an attended run with `--notes-file notes.md`, the body shows at a review gate offering accept, edit and abort, with no `r`:
  - `mint release` shows its review gate
  - `mint release regenerate <version>` shows regenerate's own review gate, as its fresh source does, not the simple confirm its `tag` and `release` sources show

  Under `-y` the gate auto-accepts on both commands
- [ ] `mint release --help` and `mint release regenerate --help` show their `--notes-file P` lines as worded in §13.1
- [ ] Given `--no-ai`, or `on_notes_failure = "fallback"` after a failed AI call, the body is the commit-subject list over `{lastTag}..HEAD`, or the maintenance-release body when that list is empty
- [ ] Given a `.mint.toml` still setting `[release].fallback`, load fails with `invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file`
- [ ] Docs:
  - The README and `mint setup`'s config reference no longer list `[release].fallback`
  - `on_notes_failure`'s description says `fallback` ships the commit-subject list
  - The setup guide's AI-transport paragraph names the commit-subject list as the one non-AI body
  - The README documents `--notes-file` on both commands

### Phase 4: Failure classification, halving and saved chunk notes
status: draft

**Goal**: When a release-notes call fails, the AI classifies the failure as SIZE or OTHER. A SIZE answer halves the failing slice, up to three times, and the notes still get written; any other answer stops. A chunked run that fails saves its finished chunk notes to a file it names, and the ✗ line names the real cause.

**Why this order**: It reacts to the calls Phase 2 makes and re-cuts a failing slice with Phase 2's packing and piece rules. The file it saves is released through Phase 3's `--notes-file`. Phase 5's floor builds its choices and guidance on the codes, cause phrases and files this phase produces.

**Acceptance**:
- [ ] Given a single-pass, chunk or merge call that fails after the transport's retry with captured output, mint makes one classify call through the release command and timeout. The call carries the SIZE and OTHER codes with their glosses, plus the failed call's stdout and stderr fenced as captured tool output. A stream over 4,000 bytes is sent as its first and last 2,000 bytes, with a line counting the bytes omitted
- [ ] Given a single-pass call whose classify answer reduces to exactly `SIZE` (in any case, after stripping surrounding whitespace, quotes, backticks and trailing punctuation), the notes re-run as a chunked run aimed at half the failed call's composed size, and are written. Any other answer stops the notes with no split, including `OTHER`, a sentence, both codes or an unknown word
- [ ] The paths that skip classification hold:
  - captured output that is empty or whitespace-only gets no classify call and takes the OTHER path
  - a classify call that fails, times out or finds no binary takes the OTHER path, and the output rendered is still the original call's
  - a notes call that times out or finds no binary gets no classify call
  - a Ctrl-C during any call propagates as a cancel and never reaches `on_notes_failure`
- [ ] Given a chunk whose classify answer is SIZE, its slice is re-cut at half that call's composed size and the release still gets merged notes. A slice is halved at most three times from its first SIZE answer, so a fourth SIZE fails it. A halving round whose fixed part leaves no room for diff also fails it, and sends no call. A SIZE answer on the merge call fails the merge without halving
- [ ] Given a chunk failure that is final, mint starts no new chunk calls and makes no further classify calls; calls already running finish and their notes are kept. Final means: OTHER, a failed classify call, no output to classify, a timeout, a missing binary, or SIZE with no halving left. A SIZE answer with halving left does not end the wave
- [ ] Given a chunked run that fails with at least one chunk's notes finished, or a failed merge, mint writes `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md` and names its full path in the failure. The file holds each finished chunk's notes exactly as returned, in chunk order, separated by a blank line. A later failure for the same tag overwrites it
- [ ] Given a saved file where some slice has no notes, the file opens with `**Notes missing:** no notes were written for {files} — add them before releasing.` and a blank line. The line lists every file with no notes, and marks files whose pieces only partly failed `(partly)`. A wave in which no chunk finished writes no file and names none
- [ ] Given each failure cause, the ✗ line uses the §12.4 phrase: `AI timed out`, `AI tool not installed`, `too large for the model, even in smaller parts`, `AI call failed`, `AI returned empty notes`, `no room for the diff under max_prompt_bytes` or `part notes too large to merge`. In a chunked run the phrase ends with where the failure happened, such as `(part 3 of 6)` or `(merging)`. The AI's captured output renders verbatim beneath it; in a chunked run that is the first final failure's output. No release-notes failure reads `AI returned empty/invalid notes after retry` or `diff too large`
- [ ] Given halving during a run, the notes line shows `Writing notes: too big for the model in one call, retrying in {n} parts` or `Writing notes: part {k} was too big for the model, retrying it as {n} smaller parts`, and the part total grows by the parts added. When the notes are written after halving, mint then says the cap is set higher than this model accepts and suggests the size of the largest call that succeeded
- [ ] Given `regenerate <version>` from a fresh source, or a version under `regenerate --all`, classification, halving and the saved file apply per version in the same way
- [ ] The README documents the saved chunk-notes file with `mint release`

### Phase 5: The remediation floor
status: draft

**Goal**: A release or regenerate whose notes end without AI output still ends somewhere useful:
- a one-keypress `c` / `e` / `a` choice at a terminal
- next moves fitted to the cause everywhere else
- a fallback warning that carries the cause, the AI's output and any saved file
- `regenerate --all` skips that name their real cause

**Why this order**: The floor is where the ladder ends, so it needs everything the ladder produces. From Phase 4 it uses the cause phrases and the saved chunk-notes file. From Phase 3 it uses three things: the commit-subject list (for `c`, which this phase extends to a regenerated version's own range), the `--notes-file` route (for the guidance), and the review gate without `r` that a non-AI body gets on each command, regenerate's included.

**Acceptance**:
- [ ] Given an attended `mint release` under `on_notes_failure = abort` whose notes fail, mint shows the ✗ line, the AI's verbatim output and the saved chunk-notes path where one exists. It then offers one choice of `c`, `e` or `a`, with the spinner suspended and `a` (abort) as the default
- [ ] Given that choice, `c` gives this release's commit-subject list, and `e` opens `$EDITOR` on the saved chunk notes when a file exists, otherwise empty. The chosen body goes to the review gate with accept, edit and abort and no `r`, and the release continues as usual
- [ ] Given the operator closes the `e` editor leaving an empty or whitespace-only body, mint returns to the `c` / `e` / `a` choice instead of reaching the review gate
- [ ] Given an unattended release (`-y` or non-TTY) under `abort` whose notes fail, mint prints the ✗ line, the AI's verbatim output and the next moves, then exits 1 without prompting. The next moves always include:
  - the `--notes-file` route: tidy the saved notes at their path, fill in any files the marker lists, and run `mint release --notes-file <path>`; when no file was saved, write the notes and release with `--notes-file`
  - `--no-ai`
  - `on_notes_failure = "fallback"` under `[release]`
- [ ] Given each cause, the next moves add:
  - size: lower `max_prompt_bytes`; use the lean command documented in the README if `ai_command` carries its own overhead; exclude generated paths with `[release].diff_exclude`
  - no room caused by operator text: shorten the outsized `[release].prompt` or `context`
  - timeout: raise `[release].timeout`
  - missing binary: install the AI tool or set `ai_command`
  - OTHER: the lean-command pointer, only when the resolved `ai_command` is not the shipped default

  A failed merge gets only the always-moves, led by its saved part notes
- [ ] Given `on_notes_failure = "fallback"` on a real run, attended or not, the commit-subject list ships with no choice and one warning. The warning carries the cause phrase, the AI's captured output verbatim beneath it, and, where a chunk-notes file was saved, its path and `mint release regenerate <version> --notes-file <path>`
- [ ] Given a dry run whose notes fail, nothing is asked:
  - under `abort` it renders the ✗ line, the verbatim output, the saved pointer and the next moves, and ends as a failed run
  - under `fallback` it previews the commit-subject list as the body that would ship, with the fallback warning naming `mint release --notes-file <path>`
  - either way, the chunk-notes file is saved as on a real run
- [ ] Given an attended `regenerate <version>` whose notes fail, whatever `on_notes_failure` holds, the same choice is offered and the run finishes in the same invocation:
  - `c` gives that version's commit subjects
  - `e` opens the editor, on the saved chunk notes where a file exists
  - `a` aborts, and stays the default
  - an empty `e` returns to the choice
  - the chosen body goes to regenerate's review gate, without `r`

  Unattended, it stops with the guidance, naming `mint release regenerate <version> --notes-file <path>` when a file was saved
- [ ] Given a version whose tag sits on its `{commit_prefix} Release {tag}` bookkeeping commit, the commit subjects `c` gives at regenerate's floor are the version's own range without that bookkeeping subject: the list that version's forward release would have shipped. A pre-tag artifact commit in the range stays listed, and an empty list gives the maintenance-release body
- [ ] Given `regenerate --all` where a version's notes fail, whatever `on_notes_failure` holds, that version is skipped and the batch carries on:
  - its skip warning and the end-of-run summary name its cause phrase
  - the skip warning carries the failed call's captured output verbatim; in a chunked run, that of the first final failure
  - a version whose wave left chunk notes names its own saved file, in the warning and in the summary, with `mint release regenerate <version> --notes-file <path>`
- [ ] `mint setup`'s AI-transport paragraph and the README describe the floor

### Phase 6: Chunked notes at the review gate and in the dry-run cache
status: draft

**Goal**: `r` on chunked notes re-runs only the merge, with the operator's nudge. `r` on single-pass notes re-runs the ladder from the size check onward. A failed `r` saves the chunk notes before it aborts. A dry run's chunked notes come back to the real run together with their per-chunk notes.

**Why this order**: It refines how Phase 2's chunked notes behave once written. Its failure route saves Phase 4's chunk-notes file and points at Phase 3's `--notes-file`, and a single-pass `r` re-enters Phase 4's ladder. Until it lands, `r` on chunked notes re-runs the whole wave.

**Acceptance**:
- [ ] Given chunked notes at an attended review gate, choosing `r` with a nudge makes exactly one AI call: the merge, carrying the existing chunk notes and the nudge. The new body returns to the gate
- [ ] Given single-pass notes and a nudge that pushes the composed prompt over the cap, `r` writes the notes as a chunked run rather than sending one over-cap call. An `r` call that fails is classified and, on SIZE, halved, as a first attempt would be
- [ ] Given a failed `r`, the run aborts before any tag whatever `on_notes_failure` holds, and `fallback`'s body never ships. Where the notes were chunked, or the ladder turned the `r` into a chunked run, mint first saves the per-chunk notes and names the file with `mint release --notes-file <path>`
- [ ] Given a single-version fresh regenerate, `r` follows the same rules. Where the notes were chunked, a failed `r` aborts after saving the file and naming it with `mint release regenerate <version> --notes-file <path>`
- [ ] Given `regenerate --all` run at a terminal without `-y`, `r` at a version's review gate follows the same rules. A failed `r` saves that version's file, names it with that version's regenerate command, and ends the batch rather than skipping the version
- [ ] Given a dry run whose notes were chunked, the following real run offers the cached notes. Using them and then choosing `r` makes only the merge call

### Phase 7: Per-verb exclusion lists and the exclusion check
status: draft

**Goal**: Release notes and commit messages each exclude a shared list plus their own level's list, and `.mint.toml` never reaches release notes. Before a forward release's AI notes are written, mint asks the AI which changed paths look out of place: it pauses on each at a terminal and prints them in an unattended run. In this phase an answer governs only the release in hand; Phase 8 records answers in `.mint.toml`.

**Why this order**: The check slots in front of the pipeline Phases 2 to 5 built. Its exclusions change the diff the size check measures, and its list condenses by Phase 2's folder rule. Its unattended output prints after the fallback list (Phase 3) and with the floor's next moves (Phase 5). The `[release]` exclusion level and keep list it checks coverage against arrive with it.

**Acceptance**:
- [ ] Given shared `diff_exclude` listing lockfiles, `[release].diff_exclude` listing `specs/` and `[commit].diff_exclude` listing `tmp/`:
  - a release's notes diff and Change Map leave out lockfiles and `specs/` but include `tmp/`
  - `mint commit`'s AI diff leaves out lockfiles and `tmp/` but includes `specs/`
  - commit's preflight still ignores every level
- [ ] Given a release range that changes `.mint.toml`, the change never appears in that release's notes diff or Change Map, whether forward or regenerated. `mint commit`'s AI diff still includes a `.mint.toml` change
- [ ] Given a forward release on the AI path, before the notes are written mint makes one AI call with no diff. The call lists each changed path not covered by the built-in excludes, either `diff_exclude` level or `[release].diff_keep`, with its change status and diff size. The list condenses by folder (for example `vendor/: 4,312 files, 38 MB`) once it would take more than a tenth of the cap
- [ ] Given `--no-ai`, `--notes-file`, a first release, an empty post-exclusion diff, or every changed path already covered, no check runs. Regenerate and `mint commit` never run it
- [ ] Given the AI suggests globs with reasons, only those matching at least one uncovered changed path survive. At a terminal each gets a pause, with the spinner suspended, showing the glob, the reason, how many changed files it matches, their total size, and any of them on the keep list. The pause offers Exclude or Keep, with Keep the default. A suggestion whose matches earlier answers already cover is skipped
- [ ] Given the operator excludes a glob, this release's notes are written without those files and the size check measures what remains. A release over the cap only because of an excluded file therefore gets one single-pass call. When the answers leave no path in the diff, the release gets the maintenance-release body
- [ ] Given the check fails, the release carries on with no suggestions after one warning, `couldn't check for files that don't belong in the notes`, with the AI's captured output verbatim beneath it where there is any, and no classify call. The failure cases are:
  - a generation failure
  - a timeout
  - a missing binary
  - an answer that is neither "none" nor yields a usable suggestion
  - a list that cannot get under the cap even condensed

  A Ctrl-C during the check cancels the run
- [ ] Given an unattended run, no pause shows and nothing is excluded. After the notes, each surviving suggestion prints with its reason, what it matches, and the `[release].diff_exclude` line that would exclude it. They print after the commit-subject list when `fallback` ships it, and with the next moves when the run stops at the floor. A dry run shows the suggestions as they would appear at the pause and asks nothing
- [ ] Given a run that executes the check, the notes line first shows `Checking for files that don't belong in the notes…`. `r` at the review gate never re-runs the check
- [ ] Docs:
  - The README's per-key tables include `[release].diff_exclude`, `[commit].diff_exclude` and `[release].diff_keep`, and the README documents the exclusion check and its pauses with `mint release`
  - `mint setup`'s config reference lists the new keys
  - The setup guide's `diff_exclude` advice sends release-only noise to `[release].diff_exclude` and keeps the shared list for what neither verb should read
  - CLAUDE.md's config seam describes the three levels, which add rather than override

### Phase 8: Recording exclusion answers
status: draft

**Goal**: The operator's Exclude and Keep answers are written into `.mint.toml`'s `[release]` table and committed on the release branch, so nobody is asked the same question again. The answers commit survives an abort or a pre-push failure, as the one deliberate exception to the surgical unwind.

**Why this order**: It records only what Phase 7's pauses produce. It is also the feature's riskiest step, so it gets its own checkpoint: it edits the operator's config in place, adds a mutation to the release, and makes the one exception to the surgical unwind.

**Acceptance**:
- [ ] Given an attended release where the operator excludes one suggestion and keeps another, after the last pause `.mint.toml`'s `[release]` table holds the first glob in `diff_exclude` and the second in `diff_keep`. Each is appended to an existing key, added to an existing table, put in a new table, or put in a new file holding only that table, as needed. Every other key, comment and piece of formatting stays as written, and nothing is added to the shared list
- [ ] mint then commits `.mint.toml` alone, staged by path, with the subject `chore(release): release-notes path answers for {tag}` whatever `commit_prefix` holds. The commit sits on the release branch and goes out with the release's push
- [ ] Given a later release, including one from another clone after the push, the kept glob's paths are never raised by the check, and the excluded glob's paths never reach the notes
- [ ] Given a Ctrl-C during the pauses, `.mint.toml` is unchanged and no commit is made
- [ ] Given an abort at the review gate or the floor, or a failure before the push, after the operator has answered, the unwind resets to the starting commit, taking the pre-tag artifact commit with it. It then re-commits the answers on top, and its narration names the answers commit as kept
- [ ] Given `--no-ai`, `fallback`, the floor's `c`, or `c` on a later regenerate of that version, the commit-subject list leaves out subjects starting `chore(release): release-notes path answers for `
- [ ] Given `--autostash` with a stashed edit to `.mint.toml` that conflicts with the answers on restore, the answers are in the committed file and the stash entry is left intact
- [ ] Docs name the answers commit as the one exception, re-committed on the starting commit:
  - the README's unwind promises, in the feature list and the surgical-unwind bullet
  - `mint setup`'s point-of-no-return line
  - CLAUDE.md's "Surgical unwind, then PONR" invariant
