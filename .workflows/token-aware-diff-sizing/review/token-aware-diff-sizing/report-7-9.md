TASK: An Exclude answer leaves its files out of this release's notes (token-aware-diff-sizing-7-9, tick-c1dcaf)

ACCEPTANCE CRITERIA:
- Given an Exclude answer on `docs/design/`, no notes call carries a `docs/design/` diff and the Change Map names no `docs/design/` path, in a single-pass and a chunked run alike (§6.5)
- Given `[release].diff_keep = ["docs/design/spec.md"]` and an Exclude answer on `docs/design/`, `docs/design/spec.md` leaves the notes too (§5.2, §6.5)
- Given a release whose composed single-pass prompt is over the cap only because of a committed 1.9 MB transcript, an Exclude answer on the transcript's path brings the size check's measure under the cap, and the notes come from one single-pass call (§7.2)
- Given Exclude answers that leave no path in the diff, the release gets the maintenance-release body with no notes call (§6.6, §7.1)
- Given a dry run that cached its notes, a real run whose operator answers Exclude at a pause does not offer the cached notes, and writes fresh notes without those files. Given every pause answered Keep instead, the real run offers the cached notes as built (§7.2, §15.2)
- Given an Exclude answer on `docs/design/` and an `r` at the review gate, the `r`'s calls carry no `docs/design/` diff (§15.1)
- Given an Exclude answer on `docs/design/`, then a Keep answer on `*.md`, which matches `README.md` and `docs/design/a.md`, `README.md` stays in the notes and `docs/design/a.md` stays out (§5.2, §6.5)

STATUS: complete

SPEC CONTEXT: §6.5 — an Exclude answer joins `[release].diff_exclude` and this release's notes are written without the files. §5.2 — the keep list adds nothing and never overrides an exclusion. §7.2 — the check precedes cache reuse and the size check, so every later step (reuse key, size measure, single-pass/chunked calls) sees the diff the answers leave; an emptied diff gets the maintenance body (§6.6, §7.1). §15.1 — `r` re-runs from the size check onward, never re-running the check. §15.2 — the cache keys on the post-exclusion diff, version and instructions.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/assemble.go:105-108 — `Assembler.Exclude` appends the answered globs (via `slices.Concat`, so the config's slice is never aliased) to the globs `excludePathspecs` (assemble.go:117-128) turns into `:(exclude)` pathspecs for the diff and both Change Map git calls (changemap.go:48-60).
  - internal/notes/exclusioncheck.go:123, 176-197 — `Check` now returns the globs answered Exclude, in answer order (`decided.excluded`); the report/preview paths return none.
  - internal/notes/select.go:214-220, 249-259 — `checkExclusions` runs the check, hands the Exclude globs to the shared Assembler and re-assembles the diff; the degenerate guard re-runs over the post-answer diff (StubBody, KindDegenerate, no AI), and the cache-key `CacheInputs.Diff`, the reuse hook and `GenerateFromDiff` all take the post-answer diff.
  - internal/engine/release.go:1016-1022 — one Assembler is shared by the Generator and the Selector (Generator copies made by `wire`/`WithProgress` keep the same `*Assembler`), so the `r` path at release.go:1340-1350 (`GenerateWithContext` → `g.assembler.AssembleDiff`, generate.go:131-137) re-assembles with the answered globs.
  - internal/notes/generate.go:200-239 — the size check, chunking and halving all run over the diff passed in, and the Change Map is built from the same Assembler, so the post-answer set governs every call.
- Notes: Keep answers never reach the Assembler, so a Keep cannot re-introduce an excluded path (AC7) and a pre-configured `diff_keep` entry cannot override an answered exclude (AC2) — both hold by construction, since only `:(exclude)` pathspecs are emitted. Keep-only runs skip re-assembly, so the cache key equals the dry run's (the answers commit touches only `.mint.toml`, a built-in exclude). `Exclude` is called once, serially, before any generation, so there is no concurrency exposure during a chunk wave. No drift from the plan's "Do" (git pathspec exclude through the notes Assembler).

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/release_exclusionanswer_test.go:112 (single-pass) and :134 (chunked) assert, through `assertNotesGitCallsExclude`, that the diff, `--name-status` and `--numstat` git calls carry `:(exclude)docs/design/`, plus call counts and the tag body.
  - AC2: :160 asserts the exact argv of both notes diff calls (no keep pathspec re-including `spec.md`).
  - AC3: :178 guards that the pre-answer diff is over `config.DefaultMaxPromptBytes`, then asserts exactly one under-cap single-pass call — a size check measuring the pre-answer diff would chunk and fail it.
  - AC4: :211 asserts no notes call and the maintenance body in the tag annotation.
  - AC5: :260 asserts no reuse prompt and one fresh call after Exclude (would fail if the reuse hook keyed on the pre-answer diff); :281 asserts the reuse prompt, no notes call and the cached body after Keep.
  - AC6: :298 asserts the third notes diff call (the `r`'s) carries the answered exclude and the `r`'s nudge rides the second call.
  - AC7: :332 asserts two pauses, the exact diff argv (only `docs/design/` excluded), README in the prompt, `docs/design/a.md` absent.
  - Unit: internal/notes/exclusionpause_test.go:161 and :177 pin `Check`'s returned Exclude globs and order, and the Keep-only empty return.
- Notes: The argv assertions are what make the engine tests discriminating (the FakeRunner returns scripted diffs regardless of pathspec); they would fail if the answers bypassed the shared Assembler on any of the diff, Change Map or `r` paths. Each test maps to one criterion; no redundancy.

CODE QUALITY:
- Project conventions: Followed (git reads via `RunInDir` at root through the runner seam; external test packages, `t.Parallel()`, exact argv assertions per CLAUDE.md test idioms)
- SOLID principles: Good — the Selector composes; the Assembler stays the single owner of exclusion pathspecs
- Complexity: Low
- Modern idioms: Yes (`slices.Concat`, `slices.Clip`)
- Readability: Good; the changed comments (select.go:145-153, 245-248; release.go:1024-1029; exclusioncheck.go:113-122; assemble.go:105) hold true against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
