SECTION: 1. Purpose and Scope · 2. The Lean Default AI Command · 3. Quote-Aware `ai_command` Parsing · 4. The Per-Call Size Cap · 5. Per-Verb Exclusion Lists · 6. The Exclusion Check · 7. Routing a Release's Notes · 8. Chunked Generation

SCOPE: One repo (`.`), base 6aa345e4f121d09d08cc16527aa594240f7bb737 (from `engine.cjs code log`), HEAD 84ec34b. The change-set is 96 task commits (1-1 … 12-2) over 277 files: 71 production files and the rest tests. Reading covered the section and every corrigendum, `code log`, and the base→HEAD diff, then the final state of the production files the section touches. Those are internal/cmdline/split.go; internal/config/{config.go,metadata.go,appendrelease.go}; internal/ai/transport.go and internal/aitransport/aitransport.go; internal/commit/{generate.go,run.go,source.go}; internal/notes/{assemble.go,pathspec.go,changemap.go,condense.go,chunk.go,generate.go,select.go,exclusioncheck.go,classify.go (cap check),remerge.go (cap check),size.go,prompt.go (ComposePrompt),resolve.go (answers-subject filter)}; internal/engine/{release.go (Release, resolveBody, exclusionCheck, answersCommit, pauses),notesstage.go,notesfloor.go (attended/asksAtTerminal),unwind.go,regenerate_fresh.go}; internal/record/commit.go (CommitFile, IsIgnored, AnswersSubject); internal/bytesize/bytesize.go; internal/presenter/{gate.go,plain.go} (pause keys, suggestion and answers lines); and cmd/mint/{main.go (signal handling),regenerate_flags.go (--notes-file scope)}. I also read go-toml v2.3.1's unstable parser for the Raw ranges that AppendReleaseGlobs relies on. Commands run (from the project root, as CLAUDE.md names them): `go build ./...`, `gofmt -l .`, `go vet ./...` and `golangci-lint run`. `go test -race ./...` was not run, because it is the whole suite. The other measurements were read-only `git` invocations over this repo's own history and index; they are listed under MEASURED and FINDINGS. `git status --porcelain` was the same before and after: only the pre-existing manifest change and the untracked review dir. The one temp excludes file in the scratchpad was removed.

MEASURED:
- "`diff.noprefix = true`, custom `diff.srcPrefix`/`diff.dstPrefix` and `diff.mnemonicPrefix` all yield `a/P b/P` headers once `--src-prefix=a/ --dst-prefix=b/` sit ahead of the range" [4-15] — measured with read-only git (2.56.0) over this repo's change-set range. Under each config, `git -c <config> diff --src-prefix=a/ --dst-prefix=b/ 6aa345e4 HEAD -- internal/notes/size.go | head -1` was run for `diff.noprefix=true`, `diff.mnemonicPrefix=true`, `diff.srcPrefix=x/` with `diff.dstPrefix=y/`, and a default-like `color.diff=never`. Each gave `diff --git a/internal/notes/size.go b/internal/notes/size.go`. The control, the same command unpinned with `diff.noprefix=true`, gave `diff --git internal/notes/size.go internal/notes/size.go` — holds. The pin covers the prefix configs only; the color and external-diff configs are not covered (see FINDINGS).
- "A tracked `.mint.toml`, one a `.gitignore` rule matches included, and an untracked or absent `.mint.toml` that no rule matches, take today's route" [8-9] — measured on this repo, where `.mint.toml` is tracked (`git ls-files --error-unmatch .mint.toml`), with a scratch excludes file holding `.*`. `git -c core.excludesFile=<scratch> check-ignore -q -- .mint.toml` → exit 1 (not ignored, so the commit route). The same rule on an untracked, absent `.mint-untracked-probe.toml` → exit 0 (ignored). The same path with no rule → exit 1. IsIgnored (internal/record/commit.go:120-131) maps exit 0 to ignored and exit 1 to not ignored — holds.

NOT MEASURED:
- "Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)" [7-7] — settling it needs a real keypress at a real TTY, in pretty raw mode and in `--plain` line mode. The project gives no way to drive a terminal, and its tests simulate input. By reading: the engine side returns the context error after the pause (internal/notes/exclusioncheck.go:183-185). The plain-mode blocked read and the second-Ctrl-C hard exit (cmd/mint/main.go:84-87) are as-built behaviour shared by every gate.
- "Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)" [8-6] — settling it means driving a release to an abort in a real repo, with git steps forced to fail, and then running the printed manual steps. That mutates a repo. The project's tests never spawn real git, its conventions give no way to stand up a scratch repo, and the project tree is not mine to mutate.
- "The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)" [7-14] — four of the five were measured clean: `go build ./...` → ok, `gofmt -l .` → empty, `go vet ./...` → ok, `golangci-lint run` → `0 issues.`. `go test -race ./...` is the whole suite, which this pass may not run; it runs once, later, over the corrected tree.
- "All five gates in CLAUDE.md pass" [12-1] — same status: the four non-test gates hold (above); the test gate is the whole suite, which runs later.
- "Every other existing test stays green, and all five gates in CLAUDE.md pass" [12-2] — the subject (git stderr beneath the unwind's warnings) is this section's §6.6. The four non-test gates hold; the suite runs later.
- "Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass" [7-15] — the subject is README documentation, which is section 18's. The four non-test gates hold; the suite runs later.
- "The existing tests pass with no test file edited … and all five gates pass" [1-6] — the subject (composition of a failed call's captured output) belongs to sections 9 and 17. The four non-test gates hold; the suite runs later.
- "Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it" [3-2] — subject is section 13 (`--notes-file`).
- "afterwards the file carries the same edits, staged if they were staged and unstaged if not" [3-2] — subject is section 13.
- "The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs" [3-2] — subject is section 13.
- "the file that path names is the one left out of the clean-tree check, the stash and the commits" [3-2] — subject is section 13.
- "afterwards the file is as it was: still untracked, or carrying the same edits, staged if they were staged and unstaged if not" [3-3] — subject is section 13.
- "In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it …" [3-4] — subject is section 13.
- "the file carries the same edits it had, staged if they were staged and unstaged if not (§13.2)" [3-4] — subject is section 13.
- "Given that file, `--autostash` with other uncommitted changes, and a failure before the push, the other changes are restored after the unwind as built …" [3-4] — subject is section 13.
- "the tag body, the changelog section and the provider release body are the same bytes (§13.3)" [3-8] — subject is section 13.
- "the existing CauseText tests (internal/notes/causetext_test.go, halvebound_test.go, halveparts_test.go, waveend_test.go, savenotes_test.go) pass unchanged" [5-14] — subject is section 12 (cause classification).
- "the existing tests in internal/engine/notes_floor_causemoves_test.go, notes_floor_sizemoves_test.go, notes_floor_nextmoves_test.go and regenerate_floor_nextmoves_test.go pass unchanged" [5-14] — subject is section 12.
- "the `NoAIBody` and `ResolveFailure` tests in internal/notes/noai_test.go (:14-193) pass unchanged" / "the `VersionNoAIBody` tests (noai_test.go:242-319) pass unchanged" [5-16] — subject is section 14.
- "Rule: the existing tests pass with their semantics untouched. ... All five CLAUDE.md gates pass" [6-11] — subject is section 15 (the review gate's `r`).
- "Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass" [6-12] — subject is section 15.
- "the existing suite stays green" [9-1] — the subject (the attended-run and chunk-notes wiring seam) belongs to sections 11 and 12.
- "The five gates in CLAUDE.md pass with no test added, removed or changed." [9-4] — subject is section 18.
- "every gate passes" [10-2] — subject is section 18.
- "Behaviour is unchanged. Every existing test stays green … All five gates in CLAUDE.md pass" [11-1] — subject is section 12.3 (regenerate's request).
- "all five gates in CLAUDE.md pass" [11-2] — subject is section 18.
- "Run against the current engine, TestRegenerateRun_Yes_AlwaysCallsPrompt … and the test passes" [11-3] — subject is section 12.3.
- "With a trial engine-side skip keyed on deps.Yes … TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes" [11-3] — subject is section 12.3.
- "The existing suite stays green … and the five CLAUDE.md gates pass" [11-3] — the subjects are sections 12.3 and 18.

FINDINGS:
- [in-scope] [contained] internal/notes/assemble.go:184 — The notes diff's argv `git diff --src-prefix=a/ --dst-prefix=b/ {range} -- . {excludes}` pins the prefixes, but it passes neither `--no-color` nor `--no-ext-diff`. So the operator's git config can still rewrite the per-file header that this change-set's new header-parsing code reads, and the comment at :145-147 ("every file header takes the "a/P b/P" form the chunker's headerPath reads") is false.
  - **Color:** under `color.ui`/`color.diff = always`, each header comes out wrapped in escape codes. Measured: `git -c color.diff=always diff --src-prefix=a/ --dst-prefix=b/ 6aa345e4 HEAD -- internal/notes/size.go | head -3 | /bin/cat -v` → `^[[1mdiff --git a/internal/notes/size.go b/internal/notes/size.go^[[m`.
  - **External diff:** under `diff.external` there is no `diff --git` header at all. Difftastic documents `git config --global diff.external difft` as its setup. Measured: the same command with `-c diff.external=echo` prints only the external tool's line; adding `--no-ext-diff` restores `diff --git a/… b/…`.
  - **Effect:** in both cases `splitFileDiffs` (internal/notes/chunk.go:280-294) finds no `"\ndiff --git "` boundary and returns the whole diff as one section. `headerPath` then returns that section's first line as its "path".
  - **Unaffected:** the Change Map's `--name-status`/`--numstat` calls (measured uncolored under `color.diff=always`).
  - **Fix:** add `--no-color` and `--no-ext-diff` to the argv at :184 and correct the comment at :144-147. The 28 argv assertions pinning `"--src-prefix=a/", "--dst-prefix=b/"` across 13 test files observe the change.
  — FAILS: for an operator whose git config sets `color.ui`/`color.diff = always` or a `diff.external` tool:
  - **The exclusion check (§6.2) is defeated.** It sends the AI a one-entry list naming a garbage path such as `ESC[1mdiff --git a/X b/XESC[m (modified, <whole diff size>)` (internal/notes/exclusioncheck.go:273-279). No suggested glob can match a real changed path, so the check either reads "none" or warns `couldn't check for files that don't belong in the notes` on every release. The design files and test changes §6 exists to catch reach the notes unflagged.
  - **Chunking loses its file structure (§8.3/§8.4).** A release over the cap is cut at line boundaries as "pieces" of one bogus file: each chunk is told it is "piece k of n of the diff for <escape-coded or external-tool line>, a file too large to read in one part", though it spans many files. Per-file packing in path order is lost.
  - **The saved chunk notes misreport.** Their missing-files marker (internal/notes/chunknotes.go:136-139) names the same garbage path.

COVERAGE:
- The lean default command is exactly the §2.1 string, and the only floor of the resolution chain — internal/config/config.go:89, :583-605 — read: AICommandFor trims to test for blank, returns the raw value, and the default is the last candidate; an explicit value at any level is untouched (§2.4).
- POSIX splitting rules (§3.1): whitespace, single quotes, double quotes escaping only `"`, `\`, `$`, backtick and newline, backslash outside quotes, backslash-newline continuation, adjacent parts joining, and `""`/`''` as one empty word; no other shell syntax — internal/cmdline/split.go:27-122 — read: `inWord` is set by a quote alone, so `--tools ""` yields an empty argument; `$`, `*`, `|` etc. fall to `appendRune`.
- The transport splits by those rules and runs the program directly through the runner — internal/ai/transport.go:333-343, :215-218 — read.
- Malformed commands fail at load at every level, whatever the verb, with the three §3.2 messages; a blank command is skipped, per the corrigendum — internal/config/config.go:844-879, called from Load at :428-430 — read: `[commit]` is checked on a release run too, and `'' --flag` hits `words[0] == ""`.
- The §3.3 whitespace-split comments were rewritten at every cited site — internal/ai/transport.go:112, :192-193, :329-332; internal/config/config.go:121-123, :576-577; internal/aitransport/aitransport.go:31 — read; `grep -i 'whitespace-split\|whitespace split\|by whitespace\|strings.Fields split'` over the three packages' non-test files finds none left about ai_command.
- Release and regenerate resolve through VerbRelease (180s, `[release]` → shared), commit through VerbCommit (60s) — internal/config/config.go:94-96, :642-670; internal/engine/release.go:1317-1322; internal/engine/regenerate_fresh.go:152; internal/commit/run.go:822 — read.
- `max_prompt_bytes`: shared-only, default 1,500,000; `<= 0` is rejected with the §4.1 message; a float or string gets the friendly type message — internal/config/config.go:70, :325, :431-433, :473, :834-839, :508-539 — read.
- The removed `max_diff_lines` names its replacement in the §4.6 wording — internal/config/config.go:550-551 — read.
- New keys take the strict path: schema fields, `typeErrorMessages` entries and SoT rows for `max_prompt_bytes`, `[release]`/`[commit]` `diff_exclude` and `[release].diff_keep` — internal/config/config.go:325-364, :473-486; internal/config/metadata.go:118-151 — read.
- Every call is bounded by the cap (§4.1):
  - single-pass size check: internal/notes/generate.go:227
  - chunk and piece calls: room = target − fixedPart, with framing numbered at the bound — internal/notes/chunk.go:369-371, :377-381, :439-444
  - merge: internal/notes/chunk.go:70-72
  - re-merge: internal/notes/remerge.go:53
  - classify call: internal/notes/classify.go:59
  - exclusion-check call: internal/notes/exclusioncheck.go:129-133
  - commit: internal/commit/generate.go:150, :164-170
  - read: each compares the composed prompt's length with the cap before sending.
- Commit over the cap goes to `$EDITOR` with `diff too large to summarise — opening editor`, including an over-cap `r` nudge — internal/commit/run.go:56, :357-359, :680-682 — read.
- Exclusion levels add up: release notes get shared + `[release]` + built-ins + the plain-mode version_file; commit gets shared + `[commit]`; neither reads the other's level; commit's preflight ignores every level — internal/engine/regenerate_fresh.go:127-133; internal/notes/assemble.go:29-32, :117-128; internal/commit/generate.go:126, :131-140; internal/commit/source.go:108-128 — read.
- `.mint.toml` is a built-in exclude beside CHANGELOG.md, on both the forward and regenerate paths — internal/notes/assemble.go:29-32 — read; both paths build the Assembler from notesExcludeConfig (internal/engine/release.go:1016, internal/engine/regenerate_fresh.go:121).
- The diff and Change Map share one exclusion set and run at the repo root — internal/notes/changemap.go:48-66; internal/notes/assemble.go:183-191 — read.
- Routing precedence (§7.1) is supplied, then first release, then empty diff, then `--no-ai`, then the AI path; the AI path (§7.2) is check, then cache reuse, then size check — internal/notes/select.go:183-245 — read: the check and reuse both sit after the degenerate and no-ai branches, and GenerateFromDiff does the size check.
- Exclude answers re-assemble the diff, and the Change Map, the cache key and later `r` calls all see them through the shared Assembler; a diff they empty gets the maintenance body — internal/notes/select.go:214-220, :249-259; internal/notes/assemble.go:106-108 — read.
- Regenerate runs no check and touches no cache (§6.1, §7.2) — internal/engine/regenerate_fresh.go:66, :100 — read: GenerateFromRange is called directly, and a grep for NoteCache/ExclusionCheck in the engine finds only release.go.
- When the check runs (§6.1): it is skipped under `--notes-file`, on a first release, for an empty diff and under `--no-ai` (precedence above), and when keep covers every path — internal/notes/exclusioncheck.go:124-128 — read.
- The check's list (§6.2): uncovered paths from the post-exclusion diff minus the keep list, each with change status and diff size; release command and timeout; no diff carried — internal/notes/exclusioncheck.go:124-129, :273-302, :342-361; internal/engine/release.go:1017, :1064 — read.
- Condensing by folder (§6.3), for both the check list and the Change Map: past a tenth of the cap, largest runs first with deterministic ties, under-share text unchanged — internal/notes/condense.go:10-12, :55-118; internal/notes/changemap.go:175-186; internal/notes/exclusioncheck.go:306-321 — read.
- Reading the answer (§6.4): "none" is case-folded; suggestions must match an uncovered path; no usable suggestion means a failed check — internal/notes/exclusioncheck.go:144-151, :249-263 — read.
- Pauses (§6.5):
  - one gate per suggestion, in order
  - a suggestion is skipped when earlier answers cover every uncovered path it matches
  - matches count paths still in the diff, kept files included and listed
  - `e`/`k` keys, Enter = keep
  - internal/notes/exclusioncheck.go:176-241; internal/engine/release.go:1188-1236; internal/presenter/gate.go:32-35 — read.
- Recording (§6.6):
  - answers are held until the last pause and written to `[release]` only — header, inline and dotted forms preserved, other bytes untouched
  - `.mint.toml` is staged by path and committed with the constant subject
  - the ignored-file route writes without committing
  - narration line
  - internal/notes/exclusioncheck.go:191-196; internal/config/appendrelease.go:35-49, :130-159, :254-358; internal/engine/release.go:1090-1133; internal/record/commit.go:107-131, :213-225 — read, including go-toml v2.3.1's Raw ranges (KeyValue spans the whole expression; InlineTable spans `{`), which addInlineKeys relies on.
- Pause, recording and unwind failures (§6.6):
  - Ctrl-C after a pause returns the cancel and records nothing
  - closed stdin aborts and unwinds without recording (it is not a generation cause, so it does not reach the floor)
  - a failed write, check-ignore or commit is surfaced with the write error or git's stderr and reverted by the unwind
  - a landed answers commit is counted and re-committed after the reset
  - a failed reset or re-commit warns with the restore step and drops "kept" and "repo clean"
  - internal/notes/exclusioncheck.go:183-188; internal/engine/notesfloor.go:32-40; internal/engine/release.go:550-560, :1999-2006; internal/engine/unwind.go:114-207, :242-245, :291-323, :353-379 — read.
- Commit-subject bodies leave out answers commits — internal/notes/resolve.go:110 — read.
- Unattended and dry runs (§6.7): an unattended run reports after the notes, also with the fallback list and with the floor's next moves, each with the `[release]` `diff_exclude` line built from the config as loaded; an attended dry run previews at the pause position; nothing is written in either — internal/engine/release.go:1063-1073, :570-573, :588-593; internal/engine/notesstage.go:86-111; internal/engine/notesfloor.go:18-26; internal/presenter/plain.go:226-241 — read.
- A failed check (§6.8): every one of the five cases warns `couldn't check for files that don't belong in the notes` with the AI's output; no classifier runs; a cancel propagates — internal/notes/exclusioncheck.go:129-151, :161-167; internal/engine/notesstage.go:51-59 — read.
- Chunked generation, map-reduce (§8.1, §8.2, §8.5): parallel chunk calls, then exactly one merge; the same Change Map in every call; at most four calls in flight; the wave drains before the merge; no concatenation fallback — internal/notes/chunk.go:28-35, :57-80, :116, :168-197 — read: `canSend` caps inFlight at 4; the run loop exits only at inFlight == 0; a failure returns an error, never joined notes.
- Packing and pieces (§8.3, §8.4): path order; fill until the next file won't fit; a file too big for a chunk becomes pieces, cut at hunks, then lines, then UTF-8 character boundaries; each piece repeats the header and names k of n; no bytes are dropped — internal/notes/chunk.go:280-294, :377-406, :419-433, :439-515 — read.
- What each call is told (§8.6): instructions, map, framing, slice and reminder for a chunk; instructions, map, framing, the notes in chunk order and reminder for the merge; the merge body passes through verbatim — internal/notes/chunk.go:542-575 — read.
- Calls that cannot be made (§8.7): ErrNoRoomForDiff when the fixed part fills the cap or the halved target; ErrMergeTooLarge for an over-cap merge — internal/notes/chunk.go:378-381, :441-444, :70-72; internal/notes/size.go:12-19 — read.
- Chunking has no setting and no consent prompt (§8.8) — internal/config/config.go:321-366 — read: no oversize key exists.
- Size narration uses the decimal format of the §16.1 corrigendum, for the check list, the pauses and the split line — internal/bytesize/bytesize.go:10-26 — read.
- The four non-test gates — measured, from the repo root:
  - `go build ./...` → ok
  - `gofmt -l .` → no output
  - `go vet ./...` → ok
  - `golangci-lint run` → `0 issues.`
