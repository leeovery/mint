# Phase 5: The remediation floor — 13 tasks

## token-aware-diff-sizing-5-1

### Task 5-1: Choose the commit-subject list or abort at the release floor

**Problem**: When an attended release's notes fail under `on_notes_failure = abort`, the run stops on the ✗ line. By then the release is fully prepared: preflight has passed, and the pre_tag hook has run with its artifacts committed. A re-run repeats every step to change one input (§12). The lean default and chunking make the floor rarer, but it is still reached: chunking exhausted, a login expired, a usage limit hit. The operator at the terminal has no way to go on without AI notes.

**Solution**: At a terminal under `abort`, when the notes fail for any cause, show the ✗ line, the AI's verbatim output and the saved chunk-notes path where a file exists. Then offer one one-keypress choice through the presenter's gate seam:
- `c`: this release's commit-subject list, or the maintenance-release body when that list is empty
- `a`: abort, the default.

Task 5-2 adds `e` to the same choice. No spinner animates while the choice waits. The chosen body goes to the review gate a non-AI body gets as built, with release, abort and edit and no `r`, and the release then continues as usual. A `-y` run never gets the choice. A failed `r` at the review gate and a Ctrl-C during the notes never reach it. The README, `on_notes_failure`'s descriptions and the setup guide's AI-transport paragraph describe the choice.

**Outcome**: An operator whose AI notes failed can release in the same run with the commit-subject list, or abort, without repeating the work the run already did.

**Acceptance Criteria**:
- [ ] Given an attended `mint release` with `on_notes_failure` unset or `abort`, whose single-pass call fails with output, mint shows the ✗ line with the AI's captured output verbatim beneath it, then offers the choice of `c` and `a`, with `a` the default (§12.1, §12.2)
- [ ] Given a chunked run that failed and saved its chunk notes, the saved file's full path shows after the output and before the choice (§11.4, §12.1)
- [ ] Given notes that fail because the call timed out, or because the AI tool is not installed, the same choice shows beneath that cause's ✗ line (§9.1, §12.1)
- [ ] Given the choice in pretty mode, no spinner animates while it waits for a keypress (§16.1)
- [ ] Given `c`, the body is this release's commit-subject list, `git log --format=%s {lastTag}..HEAD`. It shows at the review gate `Release with these notes?`, which offers release, abort and edit with no `r`. Accepting it records, tags, pushes and publishes as usual, with the list byte-identical in the tag message, changelog entry and provider release. The run ends with the release's usual success line and exits 0 (§12.2, §14.1)
- [ ] Given `c` and a commit-subject list that comes back empty or whitespace-only, the body is the maintenance-release body (§14.1)
- [ ] Given `a`, a bare Enter, or stdin closing at the choice, the run ends before any tag and exits 1, and a pre_tag artifact commit is unwound, as on an abort at the review gate (§6.6, §12.2)
- [ ] Given `mint release -y` under `abort` whose notes fail, no choice shows, no input is read, and the run exits 1 before any tag (§12.1)
- [ ] Given AI notes at the review gate and an `r` whose call fails, the run aborts before any tag, and no choice shows (§12.1, §15.1)
- [ ] Given a Ctrl-C while the notes call runs, the run aborts as a cancel, and no choice shows (§9.1, §11.5)
- [ ] The following say that under `abort` an attended release whose notes fail is offered the commit-subject list or an abort, while an unattended one stops (§14.2, §18.1, §18.2):
  - the README's `release` section
  - `on_notes_failure`'s README row
  - `on_notes_failure`'s description, as `mint setup`'s config reference renders it
  - `mint setup`'s AI-transport paragraph, which names no command flag

**Do**:
- The choice goes through the presenter's gate seam, `Prompt(gate)` (`internal/presenter/gate.go`), with the keys `c` and `a` and `a` the default (§12.2).
- The chosen body goes to the review gate a non-AI body gets as built: `FirstReleaseReviewGate()` (`internal/engine/engine.go`), which `gateForKind` (`internal/engine/release.go`) gives every Kind but `notes.KindNormalAI` (§12.2).
- The docs live in the README's `release` section and `[release]` table, in `config.MetadataRows()` (`internal/config/metadata.go`), and in the "The AI transport" text in `overview()` (`internal/setupguide/setupguide.go`) (§14.2, §18.1, §18.2).

**Context**:
> §12: the floor is where the ladder ends without AI notes. By then the release is fully prepared, so a re-run would repeat every step to change one input.
>
> §12.1, the floor by mode on `mint release`:
> - attended under `abort`: this choice
> - unattended under `abort` (`-y` or non-TTY): the next moves, then the run stops, never a prompt (task 5-3)
> - `fallback`: the commit-subject list ships with one warning and no choice (task 5-6)
> - a dry run asks nothing (tasks 5-7, 5-8).
>
> A failed `r` is not the floor; it aborts (§15.1). Phase 6 has it save its chunk notes first.
>
> §14.1: the commit-subject list is the one non-AI body, shared with `--no-ai` and `fallback` (task 3-6). Phase 8 later leaves mint's answers commit out of it. §18.1: the setup guide stays flag-free by design.
>
> As built:
> - A notes failure under `abort` returns from `resolveBody` and goes through `surfaceAndUnwind` (`internal/engine/release.go`). That renders the ✗ line with `notesFailureOutput` beneath it, then unwinds.
> - `StageFailed` sets the presenter's terminal-failure state, under which `RunFinished` suppresses the success line (`internal/presenter/plain.go`, `pretty.go`).
> - A real release's first gate is the version confirmation. It fails a non-TTY run without `-y` with `ErrNotInteractive`, so on a real release the unattended floor is reached only under `-y`.
> - The gate vocabulary, `presenter.Choice`, holds `y`, `n`, `e` and `r`. Under `-y`, `Prompt` returns the gate's `Default` and echoes the auto-accept. On a non-TTY without `-y` it returns `ErrNotInteractive`, already rendered.
> - The release engine receives neither the `-y` decision nor the stdin-is-a-TTY signal; the presenter holds both. `mint commit` receives both, as `Yes` and `StdinInteractive` in its deps (`cmd/mint/main.go`, from `presenter.DetectStartupSignals`), and its editor fallback reads them to tell an unattended run.
>
> CLAUDE.md seam 4: business logic always calls `Prompt(gate)`, and the `-y` auto-accept happens inside the presenter. "Fail loud, never hang": an unattended path that would need a human aborts with one clear message.
>
> Phase 4 names the saved file's path in the failure (task 4-10).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §9.1 The trigger; §11.4 The saved chunk-notes file; §11.5 Ctrl-C mid-wave; §12 The Remediation Floor; §12.1 What applies, by mode; §12.2 The choice; §14.1 The commit-subject list everywhere; §14.2 Removing the key; §15.1 `r` on chunked notes; §16.1 Pretty mode: one notes line, updated; §18.1 The setup guide; §18.2 The README

## token-aware-diff-sizing-5-2

### Task 5-2: Write the notes in `$EDITOR` from the release floor

**Problem**: An operator at the floor may want to write the notes by hand, or finish the chunk notes a failed wave saved (§11.4). Release's editor seam has only ever edited a body already produced (§12.2), so the floor has no route to the editor. Closing the editor empty is the usual way to back out of it. An empty body that reached the review gate would ship an empty tag message, changelog entry and provider release if accepted, since git accepts one (`printf '' | git tag -a v1 -F -` exits 0).

**Solution**: Add `e` to the release floor's choice. It opens `$EDITOR` through release's editor seam, on a copy of:
- the saved chunk notes, when a chunk or the merge failed and this run saved a file
- nothing, otherwise. That includes a run whose chunk notes could not be saved, even when a file from an earlier failure of the same tag sits at the path.

The file at the path keeps the chunk notes as the calls returned them, whatever the operator writes; the edited text lives in the run like any edit at the review gate. A non-empty body goes to the review gate with release, abort and edit and no `r`. An empty or whitespace-only body is refused, and mint returns to the `c` / `e` / `a` choice. When no editor can be launched, mint warns and returns to the choice. Any other editor failure is surfaced and aborts, unwinding a pre_tag artifact commit as any failure before the push does. The README's description of the choice covers `e`.

**Outcome**: From the floor, an operator can write or finish the notes in the editor and release with them. Backing out of the editor brings the choice back instead of an empty release.

**Acceptance Criteria**:
- [ ] Given an attended release under `abort` whose notes fail, the floor's choice offers `c`, `e` and `a`, with `a` still the default (§12.2)
- [ ] Given a chunked run that failed and saved its chunk notes, `e` opens the editor holding the saved file's contents, its missing-notes marker included where it has one. Given every part finished and the merge failed, the editor holds every part's notes, as saved (§11.3, §11.4, §12.2)
- [ ] Given a single-pass call that failed, so no file was saved, `e` opens the editor empty (§12.2)
- [ ] Given a chunked run whose chunk notes could not be saved, with a file from an earlier failure of the same tag at the path, `e` opens the editor empty (§11.4)
- [ ] Given the operator saves a non-empty body, it shows at the review gate `Release with these notes?`, which offers release, abort and edit with no `r`. Accepting it releases with the saved text byte-identical in the tag message, changelog entry and provider release (§12.2)
- [ ] Given the operator edits the opened chunk notes and saves, the file at the saved path still holds the chunk notes exactly as the wave saved them, whether the release then ships or is aborted at the review gate (§11.4, §12.2)
- [ ] Given the operator closes the editor leaving the body empty or whitespace-only, the review gate does not show, and the `c` / `e` / `a` choice shows again. Choosing `c` from it then gives the commit-subject list (§12.2)
- [ ] Given no editor can be launched, because the resolved editor is not installed, mint warns that it could not launch the editor and the `c` / `e` / `a` choice shows again (§12.2)
- [ ] Given a launched editor that exits non-zero, the failure is surfaced and the run aborts before any tag, exiting 1, and a pre_tag artifact commit is unwound, as on `a` (§6.6, §12.2)
- [ ] The README's description of the floor's choice covers `e`: it opens the editor on the saved chunk notes where this run saved a file, otherwise empty, and an empty body returns to the choice (§18.2)

**Do**:
- `e` runs release's editor seam, `EditorLauncher.Edit` behind the engine's `Editor` (`internal/engine/editor.go`), which edits a temp copy of the body it is given (§12.2).
- The README text goes in the `release` section, with the floor's choice that task 5-1 documents (§18.2).

**Context**:
> §12.2: `e` is a new write-from-scratch route, because release's editor seam had only edited an already-produced body. An empty `e` is refused because closing the editor empty is the usual way to back out of it.
>
> Corrigenda 2026-10-19 on §12.2:
> - `e` edits a copy. §11.4 defines the file's contents as each chunk's notes "exactly as the chunk calls returned them", and no step rewrites the file once saved. An edit discarded by a later abort is what the review gate's own `e` does as built.
> - An editor that cannot run: no editor launched warns and returns to the choice; any other editor failure is surfaced and aborts. Both are what the review gate does as built, and the floor's choice is the gate it returns to.
> - Every abort at the floor unwinds: `a`, an editor failure or stdin closing at the choice unwinds as any failure before the push does (§6.6), taking the pre-tag artifact commit with it. The ground is CLAUDE.md's "Surgical unwind, then PONR" invariant.
>
> §11.4 and its corrigendum: the saved file keeps the wave's work, and at a terminal the floor's `e` opens it. When the file cannot be written, the run goes on as when no file was saved, so `e` opens empty. §11.2: letting in-flight calls finish after the wave ends is what makes the attended floor's `e` cheap.
>
> As built:
> - `EditorLauncher.Edit` writes the body to a `mint-notes-*.md` temp file and launches the resolved editor (`$VISUAL`, then `$EDITOR`, then `vi`) through `RunInteractive`, bracketed by `SuspendSpinner` and `ResumeSpinner`. It reads the file back verbatim and removes it.
> - A missing editor warns `could not launch editor "…": not found` and returns `ErrEditorReturnToGate`. Any other launch failure returns wrapped for the caller to surface. `reviewGate` (`internal/engine/release.go`) re-presents itself on the first and surfaces the second.
> - After `reviewGate` returns, `Release` routes only a decline (`errGateAborted`) through `Unwind`, and returns every other gate error with no unwind. A notes failure under `abort` goes through `surfaceAndUnwind`.
>
> Phase 4 writes the chunk-notes file (task 4-10), its marker (task 4-11) and the warning when it cannot be written (task 4-12). Task 5-1 built the choice with its `c` and `a`, and task 5-9 gives regenerate's floor the same `e`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.6 Recording the answers; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.2 The choice; §18.2 The README; Corrigenda (2026-10-19, §11.4 and §12.2)

## token-aware-diff-sizing-5-3

### Task 5-3: Next moves when an unattended release stops

**Problem**: An unattended release, under `-y` from an agent, CI or a script, has nobody to offer the floor's choice to (§12.1). As built it stops on the ✗ line and the AI's output alone. Its reader, often an agent acting on the output, is told nothing of the ways forward: releasing the saved chunk notes, the commit-subject list, or the `fallback` policy. That is the dead end this feature exists to remove (§1.1).

**Solution**: When an unattended release under `abort` stops at the floor, print the ✗ line, the AI's verbatim output, then the next moves. Never prompt, and exit 1. The moves always include:
- the `--notes-file` route:
  - where a chunk-notes file was saved, tidy the saved notes at its path, filling in any files the marker lists, and release with `mint release --notes-file <path>`
  - where none was saved, including when the save failed, write the notes and release with `--notes-file`
- `--no-ai`, for the commit-subject list
- `on_notes_failure = "fallback"` under `[release]`, to ship the commit-subject list rather than stop.

Tasks 5-4 and 5-5 add the moves fitted to each cause. An attended run, which gets the choice, prints no next moves. The README and the setup guide's AI-transport paragraph describe them.

**Outcome**: An unattended release that stops at the floor tells its reader how to release anyway.

**Acceptance Criteria**:
- [ ] Given `mint release -y` under `on_notes_failure = abort` whose single-pass call fails with output, mint prints the ✗ line with the AI's captured output verbatim beneath it, then the next moves (§12.1, §12.4):
  - write the notes and release with `--notes-file`
  - `--no-ai`, for the commit-subject list
  - `on_notes_failure = "fallback"` under `[release]`, to ship the commit-subject list rather than stop.

  No prompt shows, and the run exits 1 before any tag
- [ ] Given that `-y` run with stdin a terminal, and with stdin not a terminal, the output and the exit are the same (§12.1)
- [ ] Given a chunked run that failed and saved its chunk notes at P, the `--notes-file` move says to tidy the saved notes at P, filling in any files its marker lists, and release with `mint release --notes-file P` (§11.4, §12.4)
- [ ] Given a chunked run whose chunk notes could not be saved, the move says to write the notes and release with `--notes-file`, and names no path (§11.4, §12.4)
- [ ] Given an attended release under `abort` whose notes fail, no next moves print; the choice shows instead (§12.4)
- [ ] The README's `release` section and `mint setup`'s AI-transport paragraph say that an unattended release under `abort` whose notes fail stops with next moves fitted to the cause. The setup guide paragraph names no command flag (§18.1, §18.2)

**Do**:
- The README text goes in the `release` section, with the floor's choice. The setup guide's goes in the "The AI transport" text in `overview()` (`internal/setupguide/setupguide.go`) (§18.1, §18.2).

**Context**:
> §12.4: the next moves are cause-fitted. Unattended runs print them; attended runs show the saved-file pointer and the choice. The `--notes-file` command is `mint release --notes-file <path>` while the version is not out, and `mint release regenerate <version> --notes-file <path>` once it is (tasks 5-6 and 5-11). The `on_notes_failure = "fallback"` move is for an unattended forward release only. §12.4 fixes what each move says; beyond the commands, keys and the merge example it quotes, the wording is not fixed.
>
> §11.4: the file's marker lists every file with no notes, which the move tells the reader to fill in. The corrigendum: when the file cannot be written, the run goes on as when no file was saved.
>
> §6.7, §16.3: on an unattended run that stops at the floor, Phase 7's exclusion suggestions print with these next moves.
>
> As built:
> - A real release's version confirmation fails a non-TTY run without `-y` before the notes stage, so a real release reaches this floor unattended only under `-y`. A dry run reaches it without `-y` too (task 5-7).
> - Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`).
>
> CLAUDE.md seam 3: every byte of user output goes through the presenter, and events are payload-driven. Exit idioms: `1` is a runtime failure or a user abort.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.1 The problem; §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §16.3 Follow-ups; §18.1 The setup guide; §18.2 The README; Corrigenda (2026-10-19, §11.4)

## token-aware-diff-sizing-5-4

### Task 5-4: Next moves for size and no-room failures

**Problem**: Size and no-room failures each have a specific fix, and the wrong one makes things worse (§12.4). When halving runs out, mint cannot tell a model that accepts less than the cap from a command carrying a large overhead of its own, so the guidance has to name both (§10.3). Lowering the cap fixes neither a cap already below mint's own instructions nor a merge too large: for the merge it cuts the release into more parts and does not shrink what the merge has to hold.

**Solution**: Add each cause's moves to the always-moves:
- **Size**, meaning SIZE with no halving left on a single-pass or chunk call, or a halving round with no room:
  - lower `max_prompt_bytes`, since the model accepts less than the cap
  - if `ai_command` carries a large overhead of its own, use the lean command documented in the README, without mint naming claude flags
  - exclude generated paths with `[release].diff_exclude`
- **No room caused by operator text**: shorten the operator text the call carries. That is `[release].prompt` when the release sets it, since the override replaces the default prompt and `context` with it, and otherwise `[release].context`
- **No room with neither key set**: raise `max_prompt_bytes`, since the cap is below the fixed part of mint's own instructions
- **A merge too large**, its prompt over the cap or a SIZE answer on the merge call: only the always-moves, led by its saved part notes.

**Outcome**: A run that stops because something was too big says which setting to change, and in which direction.

**Acceptance Criteria**:
- [ ] Given an unattended release under `abort` that stops because a slice's halving ran out, on a single-pass call or a chunk, the next moves add three (§10.3, §12.4):
  - lower `max_prompt_bytes`
  - if `ai_command` carries a large overhead of its own, use the lean command documented in the README
  - exclude generated paths with `[release].diff_exclude`
- [ ] Given one that stops because a halving round had no room, on a single-pass call or a part, the moves add the same three (§10.1, §12.4)
- [ ] Given those moves, the lean-command move points at the README and names none of the lean command's flags, such as `--safe-mode` or `--tools` (§9.7, §12.4)
- [ ] Given one that stops with no room for diff and `[release].prompt` set, with or without `[release].context`, the moves add shorten `[release].prompt` and do not name `[release].context`. With only `[release].context` set, they add shorten `[release].context` (§8.6, §8.7, §12.4)
- [ ] Given one that stops with no room for diff, neither `[release].prompt` nor `[release].context` set, and a `max_prompt_bytes` below the fixed part of mint's own instructions, the moves add raise `max_prompt_bytes`, and no move says to lower it (§8.7, §12.4)
- [ ] Given one that stops because the merge prompt was over the cap, or the merge call was classified SIZE, with the part notes saved at P, the moves are the always-moves only, led by the saved part notes. For example: `notes couldn't be merged; part notes saved to P — tidy them and release with --notes-file P`. No move names `max_prompt_bytes`, the lean command or `[release].diff_exclude` (§11.3, §12.4)

**Context**:
> §12.4: the size moves apply to SIZE with no halving left on a single-pass or chunk call, and to no room caused by a halving round. Both read `too large for the model, even in smaller parts` (tasks 4-1 and 4-7). The two other no-room causes both read `no room for the diff under max_prompt_bytes` (task 2-9). The moves tell those two apart by whether the release sets `[release].prompt` or `[release].context`.
>
> Corrigenda 2026-10-19:
> - on §12.4 and §8.6: a set `[release].prompt` replaces the default prompt and `context` with it, so a call carries one of the two. `ResolveInstructions` (`internal/notes/prompt.go`) returns the override without reading `context`.
> - on §8.7: a cap below the fixed part of mint's own instructions, with neither key set, is the third no-room cause. Lowering the cap would make it worse, so raising it is the one move that clears it.
>
> §10.3: when halving runs out, even small pieces come back SIZE while the small classify call fits, in a small window and under a large command overhead alike. §9.7 and §12.4: mint points at the documented lean command and never names claude flags itself. Phase 1 documents the lean command in the README (task 1-2).
>
> §12.4: a merge too large takes only the always-moves, by the cap or by the model alike, because lowering `max_prompt_bytes` cuts the release into more parts and does not shrink what the merge has to hold. The merge example is §12.4's own.
>
> `[release].diff_exclude` arrives in Phase 7. The move names it as §12.4 does.
>
> Task 5-3 built the always-moves. Task 5-7 shows these moves on a dry run, and task 5-11 on an unattended regenerate.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.6 What each call is told; §8.7 Calls that cannot be made; §9.7 An explicit `ai_command` that carries its own overhead; §10.1 When it applies; §10.3 Telling the operator; §11.3 A failed merge; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §8.7 and §12.4)

## token-aware-diff-sizing-5-5

### Task 5-5: Next moves for a timeout, a missing binary or another failure

**Problem**: A timeout and a missing binary each have one fix, which the ✗ line's phrase alone does not give (§12.4). For any other failure the AI's own message is the most specific remediation, for example `Not logged in · Please run /login` (§9.6). But a repo whose own `ai_command` carries an overhead that alone overflows the window lands on that OTHER path too: its classify call fails the same way, and nothing points it at the lean command (§9.7).

**Solution**: Add each cause's moves to the always-moves:
- **Timeout**: raise `[release].timeout`
- **Missing binary**: install the AI tool, or set `ai_command`
- **OTHER**: no move of its own, since the AI's message above names the cause. When the resolved release `ai_command` is not the shipped default, add the lean-command pointer.

**Outcome**: Every cause the floor can show comes with the move that clears it, and a repo running its own AI command is pointed at the lean one when its failure goes to OTHER.

**Acceptance Criteria**:
- [ ] Given an unattended release under `abort` that stops because a single-pass, chunk or merge call timed out, the next moves add raise `[release].timeout` (§11.6, §12.4)
- [ ] Given one that stops because the `ai_command` program is not installed, the moves add install the AI tool, or set `ai_command` (§12.4)
- [ ] Given one whose call failed with output and was classified OTHER, with no `ai_command` set at any level, the moves are the always-moves only, beneath the AI's verbatim output (§12.4)
- [ ] Given the same failure with `[release].ai_command = "llm -m gpt-4o"`, or with a shared `ai_command` and none under `[release]`, the moves add the lean-command pointer: use the lean command documented in the README. It names none of the lean command's flags (§9.7, §12.4)
- [ ] Given only `[commit].ai_command` set, the release resolves to the shipped default, and no lean-command pointer shows (§2.4, §12.4)
- [ ] Given a failure whose captured output is empty, which reads `AI returned empty notes`, the moves follow the OTHER rules (§12.4)

**Context**:
> §12.4, the phrases these causes render (task 4-1): a timeout reads `AI timed out`, a missing binary `AI tool not installed`, OTHER with output `AI call failed`, and OTHER with no output `AI returned empty notes`. A failure reaches OTHER also when its classify call fails or there was no output to classify (§9.5).
>
> §9.7: a repo whose own `ai_command` carries an overhead that alone overflows the window gets no chunk small enough to fit. Its classify call fails the same way, so the run takes the OTHER path with the AI's verbatim message. mint's guidance points at the documented lean command rather than naming claude flags (§12.4), and mint keeps no per-provider knowledge (§1.4).
>
> §2.4: the lean command applies wherever `ai_command` resolution falls through to the shipped default. The release resolves `[release]` → shared → shipped default (`config.AICommandFor` with `config.VerbRelease`), and a blank value at a level counts as unset (task 1-3).
>
> §11.6: the per-attempt timeout applies to each call on its own, and `[release].timeout` is the release verb's (task 1-4).
>
> Task 5-3 built the always-moves. Task 5-7 shows these moves on a dry run, and task 5-11 on an unattended regenerate.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.4 Out of scope; §2.4 Reach; §9.5 Edges; §9.6 Alongside the verbatim error; §9.7 An explicit `ai_command` that carries its own overhead; §11.6 Timeouts across a wave; §12.4 The ✗ line and the next moves

## token-aware-diff-sizing-5-6

### Task 5-6: Warn when `fallback` ships the commit-subject list

**Problem**: Under `on_notes_failure = fallback` the commit-subject list replaces the AI notes with nothing said: as built, the fallback body returns with no ✗ line and no warning. The operator never learns that the AI failed or why. A chunked run's saved part notes, a better body than the subject list, sit unnamed in the cache dir (§11.4, §12.1).

**Solution**: Under `fallback`, attended or not, offer no choice. Ship the commit-subject list with one warning, whatever the cause. It carries:
- the cause phrase, with its location in a chunked run
- the AI's captured output verbatim beneath it
- where a chunk-notes file was saved, its path and both commands that release it:
  - `mint release regenerate <version> --notes-file <path>`, which puts it on the version once this release is out
  - `mint release --notes-file <path>`, for a run that ends before the push, such as an abort at the review gate.

Where no file was saved, including when the save failed, the warning names no file.

**Outcome**: A release that falls back says that the AI failed and why. It also says how to release the saved part notes instead, whether this release goes out or stops at the review gate.

**Acceptance Criteria**:
- [ ] Given `mint release -y` under `on_notes_failure = "fallback"` whose single-pass call fails with output, no choice shows, and the commit-subject list ships as the release's notes. One warning carries `AI call failed` with the AI's captured output verbatim beneath it, and names no file (§12.1, §12.4)
- [ ] Given an attended run under `fallback` whose notes fail, no `c` / `e` / `a` choice shows. The same one warning prints, and the list then shows at the review gate, which offers release, abort and edit with no `r` (§12.1)
- [ ] Given a notes call that times out under `fallback`, the one warning carries `AI timed out` with no output beneath it, and the list ships (§12.1, §12.4)
- [ ] Given a 6-part chunked run under `fallback` whose part 3 fails for good with output, after other parts finished and were saved at P, the one warning carries (§12.1, §12.4):
  - `AI call failed (part 3 of 6)`
  - part 3's captured output verbatim
  - P
  - `mint release regenerate <version> --notes-file P` and `mint release --notes-file P`, with this release's version in the first
- [ ] Given a chunked run under `fallback` whose merge prompt is over the cap, the one warning carries `part notes too large to merge`, the saved path and both commands (§11.3, §12.1, §12.4)
- [ ] Given that 6-part run attended, where the operator aborts at the review gate, the run unwinds as built, and the file at P still holds the saved part notes (§11.4, §12.1)
- [ ] Given a chunked run under `fallback` whose chunk notes could not be saved, the warning names no file and neither command (§11.4)

**Context**:
> §12.1 and its corrigendum: the warning prints at the notes stage, before the review gate and the push, so whether the version will be out is not yet known. It names both commands, because an attended operator who reads it and aborts at the review gate to release the saved notes instead needs the release command. §12.4: the regenerate command is for a version already out, as after a forward release that shipped under `fallback`.
>
> §11.4: the file sits outside the repo, beside the dry-run cache but outside its pruning, so the unwind never touches it. The corrigendum: when it cannot be written, the run goes on as when no file was saved, after task 4-12's warning.
>
> §9.6: the classifier adds to the AI's verbatim output and never replaces it. §6.7, §16.3: Phase 7's unattended exclusion suggestions print after the commit-subject list when `fallback` ships it.
>
> As built:
> - `ResolveFailure` (`internal/notes/resolve.go`) builds the fallback body, which the selector returns as `notes.KindFallback` (`resolveNormalPathFailure`, `internal/notes/select.go`). `gateForKind` gives it the review gate without `r`. No ✗ line or warning renders.
> - `presenter.Warning` renders its `Output` verbatim beneath the warn line, inside the `--- output ---` delimiters in plain mode, and a warning does not set failure state.
> - The `Presenter.StageStarted` stream-discipline note forbids a `Warn` between a blocking stage's start and its completion unless the spinner is suspended.
>
> Phase 4 writes the file under `fallback` as under `abort` (task 4-10). Task 5-8 gives a dry run's warning the release command only.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.7 Unattended and dry runs; §9.6 Alongside the verbatim error; §11.3 A failed merge; §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; §16.3 Follow-ups; Corrigenda (2026-10-19, §11.4 and §12.1)

## token-aware-diff-sizing-5-7

### Task 5-7: A dry run under `abort` stops with the next moves

**Problem**: A dry run asks nothing (§12.1), so a dry run whose notes fail cannot offer the floor's choice, and as built it ends on the ✗ line alone. The operator previewing the release learns that its notes would fail, but not what to do about it. The real run would also repeat a chunked wave the dry run already paid for, unless the dry run keeps its notes.

**Solution**: A dry run whose notes fail under `abort` asks nothing, at a terminal included. It renders what an unattended run renders: the ✗ line, the AI's verbatim output, the saved chunk-notes pointer where a file exists, and the cause-fitted next moves. It then ends as a failed run. The next moves follow the dry run's own mode, so the `on_notes_failure = "fallback"` move shows only when the dry run is itself `-y` or non-TTY. The `--notes-file` route names `mint release --notes-file <path>`, since the version is not out.

The chunk-notes file is saved as on a real run, so the real run can release it with `--notes-file` instead of repeating the wave. A dry run whose chunk notes cannot be saved warns as a real run does, and its route says to write the notes.

**Outcome**: A dry run that would hit the floor shows the real run's ways forward, and keeps the wave's work for it.

**Acceptance Criteria**:
- [ ] Given `mint release --dry-run` at a terminal without `-y`, under `on_notes_failure = abort`, whose notes fail, nothing is asked and no choice shows. mint renders the ✗ line, the AI's captured output verbatim and the next moves, and the run exits 1 (§12.1)
- [ ] Given that dry run, the next moves include the `--notes-file` route and `--no-ai`, and not `on_notes_failure = "fallback"`. Given the same dry run with `-y`, or with stdin not a terminal, they also include `on_notes_failure = "fallback"` under `[release]` (§12.1, §12.4)
- [ ] Given a dry run whose chunked run failed with parts finished, the chunk-notes file is written as on a real run, at `{user cache dir}/mint/chunk-notes/{repo path, separators as dashes}/{tag}.md`. Its full path shows, and the `--notes-file` move names `mint release --notes-file <that path>` (§11.4, §12.1, §12.4)
- [ ] Given a dry run that stops because a call timed out, the moves add raise `[release].timeout`. Given one that stops because a slice's halving ran out, they add lower `max_prompt_bytes`, the lean-command pointer and `[release].diff_exclude`. Each cause adds the moves an unattended real run shows for it (§12.1, §12.4)
- [ ] Given a dry run whose chunk notes cannot be saved, the warning that they could not be saved shows with the write error verbatim, as on a real run. The `--notes-file` move says to write the notes and names no path (§11.4, §12.4)

**Context**:
> §12.1 and its corrigendum: the next moves follow the dry run's own mode, unattended meaning `-y` or non-TTY. The `fallback` move exists to ship the commit-subject list rather than stop, and at a terminal without `-y` the real run the dry run previews offers the choice rather than stopping.
>
> §12.1: the chunk-notes file sits outside the repo beside the dry-run cache, and the real run can release it with `--notes-file` instead of repeating the wave. A dry run under `fallback` saves it too (task 5-8).
>
> As built:
> - A dry run skips the version confirmation and the review gate, so a non-TTY dry run without `-y` reaches the notes stage.
> - A dry run is read-only, and its one filesystem side effect is the dry-run notes cache write (`ReleaseOptions.DryRun`, `internal/engine/release.go`).
> - A dry-run notes failure under `abort` goes through `surfaceAndUnwind` as on a real run, with nothing to unwind.
>
> Tasks 5-3, 5-4 and 5-5 built the next moves on the unattended real run. Phase 4 writes the chunk-notes file (task 4-10) and the warning when it cannot be written (task 4-12).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §11.4 and §12.1)

## token-aware-diff-sizing-5-8

### Task 5-8: A dry run under `fallback` previews the commit-subject list

**Problem**: A dry run under `on_notes_failure = fallback` previews what the real run would ship. When its notes fail, the preview has to show the commit-subject list the real run would fall back to, with the warning the real run would print (§12.1). But the real run's warning also names the regenerate command, which only serves once the version is out, and a dry run never puts it out.

**Solution**: A dry run under `fallback` whose notes fail asks nothing. It previews the commit-subject list as the body that would ship, with the fallback warning: the cause phrase and the AI's captured output verbatim beneath it, as on a real run. The chunk-notes file is saved as on a real run. The warning names a saved file with `mint release --notes-file <path>` only, never the regenerate command, since the version is not out. Where none was saved, including when the save failed, it names no file.

**Outcome**: A dry run shows the body a falling-back release would ship, why it would fall back, and how to release the saved part notes instead.

**Acceptance Criteria**:
- [ ] Given `mint release --dry-run` under `on_notes_failure = "fallback"` whose single-pass call fails with output, no choice shows, and the previewed notes are this release's commit-subject list. One warning carries `AI call failed` with the AI's captured output verbatim beneath it (§12.1)
- [ ] Given that dry run at a terminal without `-y`, with `-y`, or with stdin not a terminal, nothing is asked (§12.1)
- [ ] Given a dry run whose chunked run failed with parts finished, the chunk-notes file is written as on a real run. The warning names its full path with `mint release --notes-file <path>`, and does not name `mint release regenerate` (§12.1, §12.4)
- [ ] Given a dry run whose chunk notes could not be saved, the warning names no file and no command (§11.4)

**Context**:
> §12.1: under `fallback`, a dry run previews the commit-subject list as the body that would ship, with the fallback warning. That warning names a saved file with `mint release --notes-file <path>`, since the version is not out. §12.4: the regenerate command is for once the version is out.
>
> §11.4 and its corrigendum: when the file cannot be written, the run goes on as when no file was saved, after task 4-12's warning.
>
> As built, a dry run whose notes fall back reaches `ShowNotes` with the fallback body and closes through `finishDryRun` (`internal/engine/release.go`).
>
> Task 5-6 built the real run's warning, naming both commands. Task 5-7 saves the chunk-notes file on a dry run under `abort`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §11.4)

## token-aware-diff-sizing-5-9

### Task 5-9: Regenerate's floor offers the same choice

**Problem**: As built, a failed `mint release regenerate <version>` aborts. Its version is already out with notes, so stopping only sends the operator off to re-run with `--notes-file` (§12.3). Regenerate does not read `on_notes_failure`, and `fallback` is not for it: shipping the commit-subject list over a version's notes whenever an AI call failed would replace them with a worse body.

**Solution**: An attended single-version regenerate from a fresh source whose notes fail offers the release floor's one-keypress choice through the presenter's gate seam, whatever `on_notes_failure` holds, and finishes in the same run:
- `c`: that version's commit subjects
- `e`: the editor, through the same editor seam, on a copy of the saved chunk notes when a chunk or the merge failed and this run saved a file, otherwise empty. The file keeps the chunk notes as saved. An empty or whitespace-only body returns to the choice. An editor that cannot be launched warns and returns to it, and any other editor failure is surfaced and aborts
- `a`: abort, as-built regenerate's one outcome, and the default.

The ✗ line, the AI's verbatim output and the saved file's path show before the choice, and no spinner animates while it waits. The chosen body goes to regenerate's own review gate, with accept, edit and abort and no `r`. A failed `r` at that gate aborts, and a Ctrl-C never reaches the floor. The README's `release regenerate` section describes the floor.

**Outcome**: An operator whose fresh regenerate failed can put the commit subjects or hand-written notes on the version in the same run.

**Acceptance Criteria**:
- [ ] Given an attended `mint release regenerate v1.2.0` from a fresh source whose notes fail, with `on_notes_failure` unset, `abort` or `fallback`, mint shows the ✗ line with the AI's captured output verbatim beneath it, and the saved chunk-notes path where this run saved a file. It then offers the choice of `c`, `e` and `a`, with `a` the default. In pretty mode no spinner animates while it waits (§12.3, §16.1)
- [ ] Given `c`, where `v1.2.0`'s range `v1.1.0..v1.2.0` holds `feat: add export` and `fix: handle empty input`, and the tag sits on `fix: handle empty input`, the body is those two subjects. It shows at regenerate's review gate, which offers accept, edit and abort with no `r`, and accepting writes it to the chosen target byte-identical (§12.3, §13.3)
- [ ] Given `e` after a chunked run that saved its chunk notes, the editor opens holding the saved file's contents. Given `e` after a single-pass failure, or after a run whose chunk notes could not be saved, the editor opens empty, even when a file from an earlier failure of the same tag sits at the path (§11.4, §12.3)
- [ ] Given the editor saved with a non-empty body, it shows at regenerate's review gate with no `r`, and the file at the saved path still holds the chunk notes as the wave saved them. Given the editor closed with the body empty or whitespace-only, the review gate does not show, and the choice shows again (§12.2, §12.3)
- [ ] Given no editor can be launched, mint warns that it could not launch the editor and the choice shows again. Given a launched editor that exits non-zero, the failure is surfaced, nothing is written to any target, and the run exits 1 (§12.2, §12.3)
- [ ] Given `a`, or a bare Enter, nothing is written to any target, and the run exits 1 (§12.3)
- [ ] Given fresh notes at regenerate's review gate and an `r` whose call fails, the regenerate aborts, and no choice shows (§12.1, §15.1)
- [ ] Given a Ctrl-C while the notes call runs, the run aborts as a cancel, and no choice shows (§9.1)
- [ ] The README's `release regenerate` section says that an attended single-version regenerate whose notes fail offers, whatever `on_notes_failure` holds (§12.3, §18.2):
  - `c`, the version's commit subjects
  - `e`, the editor, on any saved chunk notes
  - `a`, abort, the default.

  It also says that an unattended one stops with guidance

**Do**:
- The choice goes through the presenter's gate seam, as on the release floor (§12.3).
- `e` runs the editor seam the release floor's `e` runs, `EditorLauncher.Edit` (`internal/engine/editor.go`) (§12.2, §12.3).
- The chosen body goes to regenerate's review gate without `r`, the one task 3-3 built for `--notes-file` (§12.3, §13.3).
- The README text goes in its `release regenerate` section (§18.2).

**Context**:
> §12.3: regenerate already runs attended with its own review gate, and stopping would only send the operator to re-run with `--notes-file`. `a` stays the default because aborting is as-built regenerate's one outcome. Unattended, it stops with guidance (task 5-11). Under `--all`, skip-and-continue stays the floor (tasks 5-12, 5-13).
>
> Corrigenda 2026-10-19:
> - on §12.2 and §12.3: an editor that cannot be launched warns and returns to the choice; any other editor failure is surfaced and aborts, as at the review gate as built. `e` edits a copy, and the file keeps the chunk notes as the calls returned them.
> - on §13.3: regenerate's review gate without `r` serves a supplied body and the floor's `c` and `e` bodies alike.
>
> §15.1: a failed `r` aborts the regenerate, and Phase 6 has it save the per-chunk notes first.
>
> As built:
> - `RegenerateRun` (`internal/engine/regenerate_interactive.go`) surfaces a `ProduceBody` failure through `surface` and aborts. The notes stage comes after the target's preflight and before `RegenerateWrite`'s gate and any changelog commit, so an abort there has nothing to reset.
> - Body production reaches the engine as `ProduceBody`, which returns only the body (`newBatchBodyProducer` in `cmd/mint/regenerate_all.go`; `RegenerateFreshBody` in `internal/engine/regenerate_fresh.go`).
> - `gateRegenerate` (`internal/engine/regenerate_write.go`) runs the fresh source's review gate through the same `reviewGate` loop and `deps.Editor` as release.
> - With `--source` and `--target` both given, a non-TTY run without `-y` asks no axis question and reaches the notes stage.
> - Regenerate has no dry run.
>
> Tasks 5-1 and 5-2 built the same choice on the release floor. Task 5-10 leaves the version's release bookkeeping commit out of `c`'s list.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §9.1 The trigger; §11.4 The saved chunk-notes file; §12.1 What applies, by mode; §12.2 The choice; §12.3 Regenerate at the floor; §13.3 How a supplied body moves through the run; §15.1 `r` on chunked notes; §16.1 Pretty mode: one notes line, updated; §18.2 The README; Corrigenda (2026-10-19, §11.4, §12.2 and §13.3)

## token-aware-diff-sizing-5-10

### Task 5-10: A version's commit subjects match its forward release's

**Problem**: Regenerate's commit-subject list runs over the version's own range, `{PreviousTag}..{Tag}`. That range holds the version's release bookkeeping commit, `{commit_prefix} Release {tag}`, the commit the tag sits on (§14.1). The forward release made that commit after its notes, so its own list never held it. Left in, `c` at regenerate's floor would put a tooling line into the version's notes. The subject cannot be matched against today's `commit_prefix` either, since a version may have been released under an earlier one.

**Solution**: Leave the version's bookkeeping commit out of regenerate's commit-subject list. It is the commit the tag sits on when that commit's subject ends in ` Release {tag}`, whatever prefix comes before it. A tag on any other commit leaves the list whole, which covers a version whose release made no bookkeeping commit. The pre-tag artifact commit stays listed, as it does on the forward range, where it lands before the notes. An empty list gives the maintenance-release body.

**Outcome**: Regenerate's commit-subject list for a version is the one that version's forward release would have shipped, whatever `commit_prefix` it was released under.

**Acceptance Criteria**:
- [ ] Given `v1.2.0` whose range `v1.1.0..v1.2.0` holds `feat: add export`, `fix: handle empty input` and the bookkeeping commit `🌿 Release v1.2.0` that the tag sits on, `c` at regenerate's floor gives `feat: add export` and `fix: handle empty input`, without the bookkeeping subject (§14.1)
- [ ] Given that version was released under an earlier `commit_prefix`, so its tag sits on `🚀 Release v1.2.0` while the repo's `commit_prefix` is now `🌿`, the bookkeeping subject is still left out (§14.1)
- [ ] Given a version whose release made no bookkeeping commit, so its tag sits on `fix: handle empty input`, every subject in the range is listed, that one included (§14.1)
- [ ] Given a range holding `chore(release): pre-tag artifacts for v1.2.0` before the bookkeeping commit, the artifact commit's subject stays listed (§14.1)
- [ ] Given a range whose only commit is its bookkeeping commit, the body is the maintenance-release body (§14.1)

**Context**:
> Corrigenda 2026-10-19 on §14.1:
> - the bookkeeping commit is made after the notes (`internal/engine/release.go:568`), while the pre-tag hook's artifact commit lands before them (`:464`, notes at `:472`). So the forward `{lastTag}..HEAD` list holds the artifact commit and never the bookkeeping one.
> - `record.BookkeepingSubject` (`internal/record/commit.go`) builds the subject from the `commit_prefix` read at run time, so today's prefix cannot recognise an older version's commit. The commit is made only when the changelog or version file changed (`record.BookkeepingWillCommit`), so the tag's position alone cannot say one was made. The tag's position and the ` Release {tag}` suffix together identify it under any prefix.
>
> §14.1: an empty list gives the maintenance-release body, so the body is never empty, as built. Phase 8 later leaves out the subjects of mint's answers commit, on regenerate as on release.
>
> As built:
> - `version.Resolution.DiffRange()` (`internal/version/resolve.go`) returns `{PreviousTag}..{Tag}`. The previous version's bookkeeping commit sits at `{PreviousTag}`, outside the range. The oldest version gets `Initial release.` with no AI, so it never reaches the floor.
> - `notes.FallbackBody` (`internal/notes/resolve.go`) builds the forward list as `git log --format=%s {lastTag}..HEAD`, and the shared selector floors an empty or whitespace-only list to `StubBody()`.
> - Release tags are annotated, so the commit a tag sits on is the one it peels to.
>
> Task 5-9 built regenerate's `c` over the version's own range.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §14.1 The commit-subject list everywhere; Corrigenda (2026-10-19, §14.1)

## token-aware-diff-sizing-5-11

### Task 5-11: An unattended regenerate stops with next moves

**Problem**: A single-version regenerate run unattended, under `-y` or on a non-TTY with `--source` and `--target` given, has nobody to offer the floor's choice to (§12.3). As built it aborts on the ✗ line alone, so the agent or script that ran it is not told how to put notes on the version.

**Solution**: Unattended, a single-version regenerate whose notes fail stops, whatever `on_notes_failure` holds, `fallback` included: it never ships the commit-subject list. It prints the ✗ line, the AI's verbatim output and the next moves, then exits 1:
- the `--notes-file` route:
  - where a chunk-notes file was saved, tidy the saved notes at its path, filling in any files the marker lists, and run `mint release regenerate <version> --notes-file <path>`
  - with no saved file, write the notes and release with `--notes-file`
- the cause-fitted moves, as on a release.

There is no `--no-ai` or `on_notes_failure = "fallback"` move; both are forward-release only.

**Outcome**: An unattended regenerate that stops at the floor tells its reader how to put notes on the version.

**Acceptance Criteria**:
- [ ] Given `mint release regenerate v1.2.0 --source fresh --target release -y` whose notes fail with output, mint prints the ✗ line with the AI's captured output verbatim beneath it, then the next moves. It writes nothing to any target, shows no prompt, and exits 1. The same holds with stdin not a terminal and no `-y` (§12.3)
- [ ] Given `on_notes_failure = "fallback"`, the run stops the same way, and the commit-subject list is written to no target (§12.3)
- [ ] Given a chunked run that saved its chunk notes at P, the `--notes-file` move says to tidy the saved notes at P, filling in any files its marker lists, and run `mint release regenerate <version> --notes-file P` for this version (§11.4, §12.4)
- [ ] Given no saved file, including when the save failed, the move says to write the notes and release with `--notes-file`, and names no path (§11.4, §12.4)
- [ ] Given any of these runs, no move names `--no-ai` or `on_notes_failure` (§12.4)
- [ ] Given a regenerate that stops because a call timed out, the moves add raise `[release].timeout`. Given one that stops because a slice's halving ran out, they add the size moves a release shows: lower `max_prompt_bytes`, the lean-command pointer and `[release].diff_exclude` (§1.3, §12.4)

**Context**:
> §12.3: unattended, regenerate stops with guidance, as release does under `abort`. Regenerate does not read `on_notes_failure`, and `fallback` stays release's: a regenerate blocks nothing, since its version is already out with notes.
>
> §12.4: once the version is out, the `--notes-file` command is `mint release regenerate <version> --notes-file <path>`. `--no-ai` and the `on_notes_failure = "fallback"` move are for a forward release, and regenerate has neither. Regenerate rides on `[release]` config (§1.3), so the cause-fitted moves name the same keys, `[release].timeout` among them.
>
> As built:
> - `validateRegenerateRequest` (`cmd/mint/regenerate_validate.go`) makes `-y` without `--target` a usage error. With `--source` and `--target` both given, a non-TTY run without `-y` asks nothing before the notes stage.
> - A failed single-version regenerate aborts through `surface` in `RegenerateRun` (`internal/engine/regenerate_interactive.go`), before any changelog commit.
>
> Tasks 5-3, 5-4 and 5-5 built the next moves on release, and task 5-9 the attended regenerate floor.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §11.4 The saved chunk-notes file; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; Corrigenda (2026-10-19, §11.4)

## token-aware-diff-sizing-5-12

### Task 5-12: A `regenerate --all` skip names its real cause

**Problem**: Under `regenerate --all`, a version whose notes fail is skipped with a generic reason and no output (`grep -n 'Label: "skipped "' internal/engine/regenerate_batch.go` → `382:	p.Warn(presenter.Warning{Label: "skipped " + tag, Message: reason})`). That breaks the rule that subprocess failure diagnostics travel verbatim, as commit's AI failure did (§17.1). An expired login across a batch reads as the same generic failure, version after version, with nothing saying what to fix (§12.3).

**Solution**: Keep skip-and-continue as `--all`'s floor, whatever `on_notes_failure` holds, attended or not. Each skipped version names its real cause phrase, with its location in a chunked run, in its skip warning and in the end-of-run summary. The skip warning carries the failed call's captured output verbatim beneath it; in a chunked run, that of the first final failure. No skip reads `diff too large`.

**Outcome**: A batch that skips versions says why each one was skipped, in the AI's own words.

**Acceptance Criteria**:
- [ ] Given `regenerate --all --source fresh` where v1.1.0's single-pass call fails with `Not logged in · Please run /login` on stdout and is classified OTHER, v1.1.0's skip warning reads `AI call failed` with that output verbatim beneath it. The end-of-run summary lists v1.1.0 with `AI call failed`, and the other versions are regenerated (§9.6, §12.3)
- [ ] Given that batch at a terminal without `-y`, or under `-y`, with `on_notes_failure` unset, `abort` or `fallback`, the version is skipped the same way, and no `c` / `e` / `a` choice shows (§12.3)
- [ ] Given a version whose 6-part wave fails for good at part 3, its skip warning reads `AI call failed (part 3 of 6)` with part 3's captured output verbatim beneath it, and its summary entry names the same phrase. Given part 5, already in flight, then fails too, the output beneath is still part 3's (§11.2, §12.3, §12.4)
- [ ] Given versions whose notes time out, find no binary, run out of halving, have no room for diff, or whose merge is too large, each skip names its phrase in its warning and in the summary (§12.3, §12.4):
  - `AI timed out`
  - `AI tool not installed`
  - `too large for the model, even in smaller parts`
  - `no room for the diff under max_prompt_bytes`
  - `part notes too large to merge`
- [ ] No skip warning or summary entry for a notes failure reads `diff too large` (§12.3)

**Do**:
- The cause phrase and the output ride the skip warning `reportSkip` emits and the summary `batchSummary` builds (`internal/engine/regenerate_batch.go`), in place of `classifyNotesFailure`'s generic reason (§12.3).

**Context**:
> §12.3: under `--all`, skip-and-continue stays the floor, whatever `on_notes_failure` holds. Its skip warning carries the failed call's captured output verbatim, as the ✗ line does. §17.1: subprocess failure diagnostics travel verbatim, and the as-built skip warning breaks that rule as commit's AI failure did.
>
> §11.2: the ✗ line renders the first final failure's cause phrase and captured output, and the skip follows it. §12.4: the cause phrases and their location suffix are Phase 4's (task 4-1). `notes.CauseText` (`internal/notes/resolve.go`) and `notesFailureOutput` (`internal/engine/release.go`) render them for the ✗ line.
>
> As built:
> - `classifyNotesFailure` maps a `ProduceBody` error to a reason string, and Phase 2 removed its `diff too large` reason (task 2-5).
> - `batchSummary` renders each skipped version as `tag (reason)`.
> - A gate decline still ends the batch.
>
> Task 5-13 names each skipped version's saved chunk-notes file.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §9.6 Alongside the verbatim error; §11.2 The first final failure ends the wave; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; §17.1 The AI's output, verbatim

## token-aware-diff-sizing-5-13

### Task 5-13: A `regenerate --all` skip names its saved chunk notes

**Problem**: Under `regenerate --all`, a version whose chunked run failed saves its finished chunk notes to a file of its own (task 4-13), but its skip does not name the file. The batch's operator cannot find the notes to finish and put on that version, so the wave's work is lost to them (§11.4, §12.3).

**Solution**: A skipped version whose wave left chunk notes names its own saved file, in its skip warning and in the end-of-run summary, with that version's own `mint release regenerate <version> --notes-file <path>`. A version whose single-pass call failed, whose wave produced nothing, or whose chunk notes could not be saved names no file.

**Outcome**: After a batch, each skipped version that kept chunk notes says where they are, and the command that puts them on that version.

**Acceptance Criteria**:
- [ ] Given `regenerate --all` where v1.1.0's wave fails with parts finished and saves its notes at P1, v1.1.0's skip warning and its entry in the end-of-run summary each name P1 with `mint release regenerate <version> --notes-file P1` for v1.1.0 (§11.4, §12.3)
- [ ] Given v1.3.0 in the same batch also saves its notes, at P3, its warning and summary entry name P3 with v1.3.0's own command. Neither version's entry names the other's file (§11.4, §12.3)
- [ ] Given a version whose single-pass call failed, or whose wave produced no notes, its skip names no file and no command (§12.3)
- [ ] Given a version whose chunk notes could not be saved, its skip names no file, and the warning that they could not be saved shows with the write error verbatim (§11.4)

**Do**:
- The file and its command ride the skip warning `reportSkip` emits and the summary `batchSummary` builds (`internal/engine/regenerate_batch.go`) (§12.3).

**Context**:
> §12.3: a version whose wave left chunk notes saves its own file, named in its skip warning and in the summary, to be fed back with that version's own `mint release regenerate <version> --notes-file <path>`. A version whose single-pass call failed, or whose wave produced nothing, names no file. §13.1: one file answers one version, so `--notes-file` with `--all` is a usage error, and each file goes on with its own version's command.
>
> §11.4: a file holding only the marker would pass `--notes-file`'s empty check and ship, so a wave in which no chunk finished saves and names no file. The corrigendum: when the file cannot be written, the run goes on as when no file was saved.
>
> Task 4-13 saves each version's file and warns when it cannot be written. Task 5-12 built the skip's cause phrase and output.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.4 The saved chunk-notes file; §12.3 Regenerate at the floor; §13.1 What it is; Corrigenda (2026-10-19, §11.4)
