TASK: ai-model-selection-3-3 — Reconcile the Commit struct doc comment to the shipped per-verb override

ACCEPTANCE CRITERIA:
- The `Config.Commit` field comment no longer contains "Deliberately NOT added for commit" or any "no per-verb(-engine) override keys exist for commit" assertion.
- The `Config.Commit` field comment states the as-built: [commit] carries per-verb ai_command / timeout overrides (resolving [commit] → shared → default) plus its Context/Prompt knobs, with diff_exclude / max_diff_lines still served shared-only from the top level.
- The `Commit` struct doc comment matches the as-built field set (Context, Prompt, and the per-verb override fields carried after Phase 1), with no residual no-per-verb-override claim and no description of non-existent fields.
- The external commit-command specification document is NOT modified (only internal/config/config.go comments changed by this task).
- No schema, accessor, wiring, or test-behaviour change is introduced (comment-only reconciliation).
- go build / gofmt -l / go vet / go test -race / golangci-lint all pass.

STATUS: Complete

SPEC CONTEXT:
specification.md "Cross-spec reconciliation (commit spec)" (lines 155-163): promoting per-verb ai_command formally reverses the commit-command spec's standing "Deliberately NOT added for commit" decision. The CODE-LEVEL reconciliation is IN SCOPE for this work unit — the moment [commit].ai_command / [commit].timeout ships, the Commit struct doc comment encoding the old "no per-verb override" contract must be updated in the SAME change (CLAUDE.md: comments stay true to as-built). Only the EXTERNAL commit-command spec DOCUMENT revision is deferrable to a separate commit-spec pass. The "In-scope vs deferrable — resolved" note (line 163) is explicit that the in-repo comment update happens here.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/config.go — Config.Commit field comment (lines 107-114); Commit struct doc comment (lines 230-240); Commit struct fields (lines 241-263). Commit was at d2816eb (Tai-model-selection-3-3), diff confined to internal/config/config.go (plus .tick/tasks.jsonl + manifest.json bookkeeping).
- Notes:
  - Config.Commit field comment (lines 107-114) now states the as-built precisely: "[commit] carries its two prompt-control knobs (Context and Prompt) ... plus the per-verb engine overrides ai_command (AICommand) and timeout (Timeout)"; resolution chain "[commit] → shared top-level → shipped default (mirroring [release])"; and correctly distinguishes per-verb keys (ai_command, timeout) from shared-only keys ("the other shared engine keys diff_exclude / max_diff_lines stay shared-only and serve commit from the top level (these two keys have no [commit] override)"). Matches AC2 exactly.
  - Commit struct doc comment (lines 230-240) enumerates exactly the as-built field set: Context, Prompt (prompt knobs, raw TOML strings, default empty, with the ResolveCommitPrompt fail-loud note preserved) AND AICommand (*string) / Timeout (*time.Duration) per-verb overrides — "see their per-field comments below." This matches the four-field struct (Context, Prompt, AICommand, Timeout at lines 242, 243, 251, 262) exactly; no non-existent field is described, no field omitted. Matches AC3.
  - grep for "Deliberately NOT added for commit" and "per-verb-engine override keys exist for commit" / "no per-verb" in config.go: NO MATCH. The stale OLD claims (and the interim Phase-1/2 "this REVERSES the standing-spec ... remain for Phase 3" scaffolding text) are fully gone. AC1 satisfied.
  - External commit-command spec document (.workflows/mint-release-tool/specification/commit-command/specification.md) still carries "Deliberately NOT added for commit" and was NOT in the 3-3 diff — correctly deferred. AC4 satisfied.
  - No schema/accessor/wiring change in the diff — comment text only. AC5 satisfied.

TESTS:
- Status: Adequate (N/A by design — comment-only reconciliation)
- Coverage: This is a comment-only reconciliation; no new behavioural test is warranted and none was added. Verification is the documented grep checks (all pass: stale claims absent, as-built per-verb overrides named, struct comment matches current fields) plus the gates. Confirmed no existing test asserts the Commit doc-comment text (grep of internal/config/*_test.go for the old phrases: no match), so the edit perturbs no test pin; Phase 1's AICommandFor / TimeoutFor / decode tests remain the behavioural proof and are untouched.
- Notes: Correct call to add no test — pinning prose comment text would be over-testing (brittle, tests implementation-detail wording, not behaviour). Per the project test idioms this would be exactly the kind of redundant assertion to avoid.

CODE QUALITY:
- Project conventions: Followed. CLAUDE.md "Comments" contract (heavy WHY-comments kept true-to-as-built in the same change) is precisely what this task discharges. golang-documentation idioms upheld: the Commit struct doc comment begins with the type name ("Commit holds ..."), field comments lead with the field name ("AICommand is ...", "Timeout is ..."). Comments stay model-agnostic and describe role + the absent-vs-explicit-zero pointer rationale.
- SOLID principles: N/A (no code change).
- Complexity: N/A (no code change).
- Modern idioms: N/A (no code change).
- Readability: Good. The reconciled comments are accurate, internally consistent, and correctly cross-reference [release] as the mirror; the per-verb-vs-shared-only distinction is stated unambiguously. The Commit struct doc comment correctly defers field-level detail to the per-field comments rather than duplicating it (DRY for prose).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/config/config.go:237 — Commit struct doc comment references "ResolveCommitPrompt at the point of use (assembly, 1-2)"; the AICommand field comment at line 247 was deliberately de-versioned to "the resolver" while the Config.Commit field comment (line 109) names accessors AICommandFor/TimeoutFor. Optionally drop the bare task-id "(assembly, 1-2)" from the doc comment so the prose does not carry internal plan task-ids that mean nothing to a future reader of the shipped code. Pure wording, zero logic impact.
