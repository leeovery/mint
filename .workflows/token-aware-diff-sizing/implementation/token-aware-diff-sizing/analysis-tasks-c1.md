# Analysis Tasks: Token Aware Diff Sizing (Cycle 1)

## Task 1: Give `mint release`'s attended-run and chunk-notes wiring a tested seam
severity: medium
sources: architecture

**Problem**: This feature added three `ReleaseDeps` fields that change what an operator gets.
- `Yes` and `StdinInteractive` decide whether a run is offered the exclusion check's pauses and the notes floor's choice (`ReleaseDeps.attended` / `asksAtTerminal`, internal/engine/notesfloor.go:18-26).
- `ChunkNotes` keeps a failed wave's part notes. A nil store means no saver (internal/engine/chunknotes.go:27-31).

`runRelease` (cmd/mint/main.go:284-334) fills all three inline from os globals (`opts.Yes`, `signals.StdinInteractive`, `notescache.NewUserChunkNotes(os.UserCacheDir)`). No test drives it past flag parsing, and every engine test builds `ReleaseDeps` by hand. So a refactor of that literal that drops or crosses a field compiles and passes the whole suite:
- If `deps.Yes` stops tracking `opts.Yes`, a `mint release -y` at a terminal pauses on every suggestion. The -y presenter answers each pause with its default, Keep, and mint writes, commits and pushes `diff_keep` entries unattended. The floor's choice auto-answers Abort instead of printing the next moves.
- If `StdinInteractive` is dropped, every attended release runs as unattended: no pauses and no floor choice.
- If `ChunkNotes` is dropped, a failed chunked wave saves nothing and gives no "could not be saved" warning.

The operator notices after the fact, through a config commit they never chose or notes that went missing. Regenerate already has this seam from this plan: `runRegenerate` is a thin shell over `regenerate(ctx, p, r, req)` (main.go:129-149), and cmd/mint/regenerate_floor_test.go and regenerate_exclusioncheck_test.go pin how -y and the stdin signal reach its engine. Release did not get one, and it is the only verb that records and commits exclusion answers.

**Solution**: Split `runRelease` the way `runRegenerate` is split.
- **The shell** parses flags, detects the startup signals, and builds the presenter, the exec runner, the note cache and the chunk-notes store from os globals.
- **The seam**, `release(...)`, takes the presenter, the runner, the parsed options, the stdin signal, the note cache and the chunk-notes store as parameters, and builds `ReleaseDeps` (Mutator and Releaser included) from them.

The two stores are parameters because otherwise a cmd-level test through the seam would write into the developer's real user cache dir. `newNoteCache()` would write it on a successful notes call and the chunk-notes store on a failed chunked run (main.go:325-326). Tests root everything in `t.TempDir`.

Pin the mapping with cmd-level tests through the seam, scripted with `runner.FakeRunner` and recorded with `presentertest.RecordingPresenter`:
- a -y run whose check suggests a glob presents no pause and writes no `.mint.toml`;
- a non-TTY run offers no floor choice and prints the next moves;
- a failed chunked run saves its part notes to the injected store and names the file.

Derivation: regenerate's split in this plan is the precedent for these same signals. `mint release` behaves exactly as it does today. `regenerate(...)` keeps its hard-wired chunk-notes store, since no test reaches it.

**Outcome**: If release's wiring drops or crosses `Yes`, `StdinInteractive` or `ChunkNotes`, a cmd-level test fails, as the same signals already do on regenerate.

## Task 2: Build the setup guide's transport-paragraph defaults from the config constants
severity: low
sources: duplication

**Problem**: The setup guide's "The AI transport" paragraph (internal/setupguide/setupguide.go:156-163) takes the shipped command from `config.DefaultAICommand`. It hand-types the three other shipped defaults it states: "default 180 for release, 60 for commit" and "default 1,500,000".

The same guide renders its config reference from `config.MetadataRows()`, whose Default cells derive from `config.DefaultReleaseTimeout`, `config.DefaultCommitTimeout` and `config.DefaultMaxPromptBytes` (internal/config/metadata.go:118-119, :138, :153). Change one of those defaults and `mint setup` emits a guide that contradicts itself: the reference shows the new value and the prose the old. This feature moved release's timeout from 60 to 180, and §18.1 had to name the paragraph's stale `default 60` for removal by hand.

The agent reading the guide is the one configuring the repo. If it trusts the prose, it can leave a slow `[release].ai_command` under a timeout it believes is 180s, and every notes call then hits the fatal per-attempt deadline. `TestGuide_AITransportParagraphNamesBothShippedTimeouts` and `TestGuide_AITransportParagraphCoversThePerCallCapAndChunking` pin the same literals, so they stay green against stale prose.

**Solution**: Build the paragraph's three numbers from the config constants, as the paragraph already does for `config.DefaultAICommand` and as metadata.go does for the reference cells: the two timeouts in whole seconds, and the cap as `strconv.Itoa(config.DefaultMaxPromptBytes)`. The prose then reads `default 1500000` where today it reads `default 1,500,000`.

Ungrouped digits are the settled form, for three reasons:
- the guide's own config-reference cell writes the value that way;
- it is the literal an agent writes in `.mint.toml`;
- reusing `notes.groupThousands`, which is unexported, would mean a notes import or a new home for a six-line formatter, all for one number in prose.

The two prose tests build their expected text from the same constants. The paragraph still names release's 180s default beside commit's 60s, as §18.1 requires, and the rest of the guide stays byte-identical.

## Task 3: Corrections
severity: corrections
sources: standards

**Problem**: CLAUDE.md:42, the "Surgical unwind, then PONR" invariant, still names the answers commit as the only thing an unwind keeps. Since the 2026-10-21 corrigendum to §6.6 (task 8-9), the unwind also keeps the answers' write when git ignores an untracked `.mint.toml`: `a.edit = nil` before the unwind (internal/engine/release.go:1105-1116). The README (line 117) and the unwind.go header (internal/engine/unwind.go:11-14) already say so. §18 and CLAUDE.md's own as-built rule require the contract to say so too. A maintainer working from the contract sees a pre-push write that survives an unwind and reads it as a leak. Adding it to the unwind brings back the dead end the corrigendum closed: a repo that ignores `.mint.toml` is asked the same questions again after every abort.

**Solution**:
- CLAUDE.md:42: replace "The one exception is the exclusion check's answers commit: the unwind resets past it with the rest, then re-commits the answers on the starting commit and narrates it as kept." with "The one exception is the exclusion check's answers: the unwind resets past their commit with the rest, then re-commits them on the starting commit and narrates it as kept; where git ignores an untracked `.mint.toml`, the uncommitted write is left in place."
