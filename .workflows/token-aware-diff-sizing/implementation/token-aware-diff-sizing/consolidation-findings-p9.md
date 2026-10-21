# Consolidation Findings: Token Aware Diff Sizing (Phase 9)

## Findings

None.

## Comment Corrections

- cmd/mint/main.go:337-342: runCommit's doc says it mirrors runRelease's seam wiring. After this phase runRelease builds only the presenter and exec runner, and the Mutator is built in release(), so the cross-reference no longer describes runRelease. The rest of the sentence restates the function body.
  OLD:
  ```
  // runCommit wires the production seams and runs the bare `mint commit` generate-and-
  // commit thread for a parsed `mint commit` invocation, returning the process exit
  // code. It mirrors runRelease's seam-wiring idiom: construct the presenter ONCE via
  // NewForStartup, one ExecRunner backing the read-side staged-diff seam, and the
  // lock-resilient git Mutator (git_safe) as the commit sink over that same runner. All
  // orchestration lives in the testable commit.Run; main stays thin.
  ```
  NEW:
  ```
  // runCommit wires the production seams and runs the bare `mint commit` generate-and-
  // commit thread for a parsed `mint commit` invocation, returning the process exit
  // code. All orchestration lives in the testable commit.Run; main stays thin.
  ```

## Spec Defects

### S1: §18's unwind-exception doc requirements name only the answers commit, after the §6.6 corrigendum added the ignored-file write
- **Claim**: §18.3 (spec line 892): "It is rewritten to carry the one exception: the exclusion check's answers commit, which the unwind re-commits on the starting commit (§6.6)." §18.1 (line 878) asks for the same wording in the setup guide's point-of-no-return line ("the exclusion check's answers commit stays, re-committed on the starting commit"). §18.2 (line 884) asks for it in the README's unwind promises.
- **Observed**: The 2026-10-21 corrigendum (spec line 1004) added a §6.6 bullet (line 321): when `.mint.toml` is untracked and ignored, "An abort or failure after answering keeps the write, as it keeps an answers commit." The code does this: internal/engine/release.go:1103-1115 sets `a.edit = nil` with no commit when `record.IsIgnored` holds. This phase's task 9-3 rewrote CLAUDE.md:42 to name both survivals ("where git ignores an untracked `.mint.toml`, the uncommitted write is left in place"), which goes beyond what §18.3 asks for. README.md:117 and internal/engine/unwind.go:11-14 also name both. The sites written to §18's text still name only the commit: internal/setupguide/setupguide.go:263-265 ("The one exception: after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit."), README.md:30 and README.md:351. They now contradict CLAUDE.md:42 and README.md:117.
- **Read**: spec stale. The corrigendum corrected §6.6 but did not carry the ignored-file survival into §18.1-§18.3's doc requirements. The guide's pipeline line and README:30/:351 follow that stale wording, and an agent reading `mint setup` is told an ignored `.mint.toml` write is unwound when it is kept.
