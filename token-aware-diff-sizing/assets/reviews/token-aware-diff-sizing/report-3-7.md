TASK: Regenerate refuses an unusable notes file before it resolves the publisher (token-aware-diff-sizing-3-7, tick-007544)

ACCEPTANCE CRITERIA:
- `mint release regenerate 1.4.0 --notes-file P` with no `origin` remote or a non-github one, and P empty, whitespace-only, missing or `""`, reports the one `notes` refusal (`--notes-file P is empty`, or `reading --notes-file P: <cause>`), shows no `publish skipped` warning, runs neither the tag lookup (`git tag --list`) nor the remote read (`git remote get-url origin`), and exits 1 (§13.2, §13.4)
- `mint release regenerate 9.9.9 --notes-file P` with P unusable and no `v9.9.9` tag reports the `notes` refusal, not `no tag v9.9.9 found`, and exits 1, not 2 (§13.4)
- With a usable P given as a relative path, regenerate resolves it against the directory mint was run from, asks no source question, and ships the body read at startup to the chosen targets byte-identical, even when P is rewritten later in the run (§13.1, §13.2, §13.3)
- Without `--notes-file`, regenerate reads no file and runs as before: the publisher resolves, a non-github origin still warns `publish skipped`, and the source question is asked (§13.1)

STATUS: complete

SPEC CONTEXT: §13.2 has mint read the --notes-file once at startup, before any stash, check or gate, and hold the body in memory. §13.4 refuses an empty/whitespace-only file (`--notes-file P is empty`), a missing/unreadable file (`reading --notes-file P: <cause>`) and an empty path (fails as a missing file) before any step runs, exit 1; usage errors (--no-ai, --all, --source) stay exit 2. §13.1 resolves a relative path against the directory mint was run from. §13.3 ships the supplied body byte-identical and skips the source question.

IMPLEMENTATION:
- Status: Implemented (with a sound divergence: the read sits even earlier than the plan named)
- Location:
  - cmd/mint/main.go:147-153 — new `regenerate` seam reads the notes file through `engine.ReadNotesFile` as its first step, ahead of `gitrepo.ResolveRoot` (:158), `config.Load` (:163), validation (:169), `ResolveReleaseBranch` (:181), and therefore ahead of `version.ResolveRegenerateTarget` (:219) and `resolveRegeneratePublisher` (:238) in `runRegenerateSingle`; a refusal returns `exitCode(err)` → 1
  - cmd/mint/main.go:218, :244 — `runRegenerateSingle` takes the read body and sets `runReq.Supplied`
  - internal/engine/release.go:964-973 — exported `ReadNotesFile` wraps `readNotesFile` (:978-991) and surfaces the error on the presenter at stage `notes` via `surface`, returning the exit-1 `*AbortError`; `Release` uses the same entry point at :354-358
  - internal/engine/regenerate_interactive.go:125-131 — `RegenerateRunRequest` carries `Supplied *string` beside `NotesFile`; `NotesFileSet` removed; `RegenerateRun` no longer reads (:163-166 overrides the source, :270-275 returns the supplied body); `notesExclusion` still uses `req.NotesFile` (:199)
  - cmd/mint/regenerate_run.go:30-37 — `regenerateRunFlags` passes only `NotesFile`
- Notes: The plan placed the read at the head of `runRegenerateSingle`; the code reads at the head of `regenerate`, before root/config/branch resolution. This is strictly earlier, still after the parse-time usage errors (--no-ai/--all/--source, exit 2), and matches release's order (read before `ResolveRoot`). --all with --notes-file is rejected at parse, so the earlier read never runs for the batch path. Relative paths read via `os.ReadFile` against the process cwd, satisfying §13.1. Field and doc comments on `RegenerateRunRequest.NotesFile`/`Supplied`, `regenerate`, `runRegenerateSingle` and `ReadNotesFile` hold true against the code.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: cmd/mint/regenerate_notesfile_test.go:126-153 drives `regenerate` with a FakeRunner seeded for root/branch/tag/remote and a RecordingPresenter, across no-origin and non-github origins × empty, whitespace-only, missing and `""` paths; `assertOnlyNotesRefusal` (:80-95) pins exit 1, zero invocations (no tag list, no remote read), and exactly one `StageFailed{Name:"notes", Message:...}` (so no `publish skipped` warn). Fails under the old order (warn + tag/remote invocations would appear).
  - AC2: :155-172 uses version 9.9.9 with tags v1.3.0/v1.4.0 seeded; under the old order it would exit 2 with `no tag v9.9.9 found`, so the exit-1 + refusal assertion catches regression.
  - AC3: :192-230 chdirs to a run dir distinct from root, passes `notes.md` relative, rewrites the file when `git remote get-url origin` runs (after startup), and asserts prompt subjects == ["notes"] (no source question) and `gh release edit` stdin equals the startup body byte-for-byte (body carries trailing spaces and blank lines). Under the old order the rewritten body would ship, failing the test.
  - AC4: :232-257 asserts the tag lookup and remote read run, exactly one `publish skipped` warn, and a `source` prompt; internal/engine/notesfile_refusal_test.go:119-135 pins that `ReadNotesFile(p, "", false)` reads nothing and shows nothing.
  - Engine: `notesFileRunReq` (internal/engine/regenerate_notesfile_test.go:32-44) now reads through `engine.ReadNotesFile` and sets `req.Supplied`; the old `TestRegenerateRun_UnusableNotesFile_IsRefusedBeforeAnyStep` is replaced by `TestReadNotesFile_UnusableNotesFile_IsRefusedOnTheNotesStage` (internal/engine/notesfile_refusal_test.go:100-117), which also covers the unreadable (directory) case; `TestReadNotesFile_UsableFile_ReturnsItsBodyByteIdentical` (:137-155) pins byte identity.
- Notes: The cmd test drives the new `regenerate` seam rather than `runRegenerateSingle` as the plan suggested; that covers the whole startup order (root, config, branch, tag, publisher), which is the stronger proof. No redundant or over-mocked tests.

CODE QUALITY:
- Project conventions: Followed — external commands only via the injected runner, output only via the presenter, refusal surfaced once by the engine with the cmd layer adding no narration, sentinel/AbortError mapped through `exitCode`, tests use FakeRunner + RecordingPresenter with exact argv/event assertions.
- SOLID principles: Good — the `regenerate` seam separates production seam construction (`runRegenerate`) from orchestration, giving the cmd layer a testable entry point.
- Complexity: Low
- Modern idioms: Yes (t.Context, t.Chdir, slices.Contains/Equal)
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
