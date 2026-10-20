## Attempt 1

ISSUES:
- cmd/mint/regenerate_all.go:136 (`engine.RegenerateFreshRegenerator(r, nil, root, cfg, res, merged)`) and cmd/mint/regenerate_run.go:106 (`produce(source, res, merged)`) are how production `r` reaches the re-merge, and no test covers them.
  - The cmd tests (cmd/mint/regenerate_all_test.go:283-313, cmd/mint/regenerate_run_test.go:110-118) pass only `nil` and only check that the result is nil or non-nil.
  - The engine tests (internal/engine/regenerate_remerge_test.go `freshRunOf` and the batch closure in `TestRegenerateAll_RegenOnAChunkedVersion_...`) copy the closure and inject `tr`, which bypasses `resolveFreshTransport`.
  - The failure this misses: an edit that passes `nil` instead of `merged` in either producer, which is likely when 6-3 reworks how the regenerator holds merge inputs. Production `r` on chunked notes would then silently re-run the whole wave (N chunk calls plus a merge, minutes long) with every gate green. Nobody notices until an operator sees a slow `r` or a bill.
  - The re-merge running through the release `ai_command`, which the task's Do names explicitly, is also unpinned. 6-1 pinned `leanClaudeArgs` on its re-merge.
  FIX: Add cmd-level tests in cmd/mint for both producers.
  - Build the regenerator with `newRegenerateRegeneratorProducer(f, cfg, root, res)(engine.RegenerateSourceFresh, &notes.MergeInputs{})`, and likewise with `newBatchRegeneratorProducer(f, cfg, root)(engine.RegenerateSourceFresh, res, &notes.MergeInputs{})`.
  - Seed only `claude` on the FakeRunner with a valid body, then call `.Regenerate(t.Context(), "lead with the export feature")`.
  - Assert exact results: no error; `f.Invocations()` is exactly one `claude` invocation whose argv is the release default command's args; its stdin contains the nudge and no `diff --git `; the returned body is byte-identical to the seeded one.
  - A dropped `merged` sends the regenerator down the range path, whose first call is an unseeded `git diff`, so the test fails.
  ALTERNATIVE: One end-to-end test through the existing `regenerate(t.Context(), rec, f, parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release"))` harness, seeded like `TestRegenerate_AttendedFreshFailure_...`:
  - `seedRegenStartup`, plus a `.mint.toml` with `max_prompt_bytes = 20000` in root, and an over-cap diff like `seedOverCapFreshVersion`.
  - `claude` seeded with the chunk notes, the merge, then the re-merge; choices `[ChoiceRegen, ChoiceYes]`.
  - Assert N+2 claude calls, each with the release argv, one diff assembly, and a gh write whose stdin is the re-merge body.
  - It pins the full production path including main.go:256, but covers only the single-version producer and needs heavier seeding. I recommend the unit-level pair: it targets exactly the two untested lines.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- internal/engine/regenerate_fresh.go:120 — false: the body and a re-running `r` share the same range, and a re-merge has no range at all.
  OLD: // release transport — only the range/context differ.
  NEW: // release transport.
- internal/engine/chunknotes.go:44-45 — restates the one-line body. It also overclaims: `k.saver()` is nil, and saves nothing, when no store is wired.
  OLD: // sinks is where a regenerate's fresh generation, inside the notes stage progress,
// saves a failed chunked run's notes and hands a written one's merge inputs to k.
  NEW: 

NOTES:
- I ran the gates myself: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race` on internal/engine, cmd/mint and internal/notes, and `golangci-lint run` (0 issues). All pass, including the five new tests.
- The executor claims all five new tests fail with the re-merge branch disabled. That holds by reasoning: the range path adds three git calls and six AI calls, which breaks every count and prompt-equality assertion.
- Some rewrapped comment lines were left overlong. This is formatting, not content, and 6-1 left the same pattern, so non-blocking:
  - cmd/mint/regenerate_all.go:126 (141 cols)
  - internal/engine/regenerate_interactive.go:281 (114 cols)
  - internal/engine/regenerate_write.go:133 (123 cols)
  - cmd/mint/regenerate_run.go:85 is left as a short ragged line.
- Constraint for 6-3 (tick-1e4b73, "a further `r` after an `r` the ladder chunked re-merges", on release and regenerate). Both `perRunRegenerator` (release.go) and `RegenerateFreshRegenerator` bind `merged` immutably when they are built, and their generators carry no `WithMergeInputs`. 6-3 will have to change both binding sites.
- cmd/mint now imports internal/notes, in regenerate_all.go and regenerate_run.go, only to name the opaque `*notes.MergeInputs` it passes back to the engine. That is acceptable for the thin-entry layer, but the type now crosses engine → cmd → engine.
- `RegenerateFreshRegenerator` now takes six parameters (it had five).
- Two new tests index merges using only a total-prompt-count guard: `tr.merges[2]` in the second-`r` test and `v13.merges[1]` in the `--all` test. A misclassified prompt would panic instead of failing cleanly. Minor.
- No dispatch content arrived beyond the enumerated inputs.
