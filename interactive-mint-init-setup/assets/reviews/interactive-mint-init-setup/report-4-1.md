TASK: 4-1 — Reconcile the Configuration + Commands sections with the minimal template

ACCEPTANCE CRITERIA:
- The `## Configuration` intro states `mint init` writes a minimal `.mint.toml` (empty body + header comment pointing to GitHub docs and `mint setup`) and no longer contains "commented `.mint.toml`".
- The embedded full commented-template TOML block is replaced with the minimal template (empty body + dual-pointer header) OR removed; no fenced TOML block shows the old commented-every-key scaffold.
- If replaced, the shown header carries both pointers (GitHub docs + `mint setup`), matching `initgen.MintTOML()`.
- The Commands `### init` block describes a minimal `.mint.toml` (empty body + header pointer) and no longer says "commented `.mint.toml` (every key shown at its default…)".
- Configuration intro and Commands `### init` line AGREE on the framing.
- Per-key reference tables confirmed to declare every config key + default; missing rows added, otherwise left intact.
- README still builds as valid Markdown (no broken fences/anchors).

STATUS: Complete

SPEC CONTEXT:
Spec sections "Generated config: strip to minimal" and "README — config reference verification". Phase 3 changed `initgen.MintTOML()` to emit a stripped minimal file: empty body + a short header comment whose pointers are the cold-arrival recovery net (GitHub docs for the human config reference, `mint setup` for AI-assisted setup). The README was the pre-pivot home of the only in-repo config docs; post-pivot the binary (`mint setup` SoT table) + GitHub docs/README are the doc sources, so the old embedded commented-template block and "commented `.mint.toml`" framing became factually false and had to be reconciled. The per-key reference tables remain the authoritative HUMAN config reference and are the surface the Task 4-3 tripwire checks; README is allowed to lightly duplicate the SoT (accepted — it is the human GitHub-browsing surface).

IMPLEMENTATION:
- Status: Implemented (correct; replace-path chosen)
- Location:
  - README.md:184 — Configuration intro now reads "`mint init` writes a **minimal** `.mint.toml` at the repo root: an empty body plus a short header comment that points to the GitHub docs (this human config reference) and to `mint setup` (AI-assisted setup). The file is fully optional — every key has a compiled default…". No "commented `.mint.toml`".
  - README.md:186-193 — the embedded fenced ```toml block was REPLACED with the minimal template. Its 6 lines are byte-identical to what `initgen.MintTOML()` emits (initgen.go:39-44): the fully-OPTIONAL header, the github.com/leeovery/mint docs pointer (line 191), and the `mint setup` pointer (line 192). Both pointers present → AC for the replace path satisfied.
  - README.md:195 — explicit hand-off: "The per-key reference tables below are the authoritative human config reference — every key, its level, and its default."
  - README.md:70 — Commands `### init` line now reads "writes a minimal `.mint.toml` (empty body plus a short header comment pointing to the GitHub docs and `mint setup`) and a `release` shim at the git-resolved repo root. Idempotent: existing files are skipped unless `--force`." Shim + idempotency framing retained; only the `.mint.toml` description changed.
- Notes:
  - The Configuration intro (line 184) and the Commands init line (line 70) AGREE — both say "minimal `.mint.toml`", empty body + header pointer to GitHub docs and `mint setup`. AC (c) met.
  - No contradictory stale framing anywhere in the README: grep for "commented" returns zero matches; grep for "every key shown / optional keys commented / scaffold / inline key comment / commented example / writes a commented / full template" returns zero matches. AC (d) met.
  - Markdown integrity: the single fenced ```toml block (186-193) opens and closes correctly; the in-section anchor links (`#install`, `#the-ai-transport`) used near this section resolve to real headings. No broken fences or dangling anchors introduced.

TESTS:
- Status: Adequate (no Go test owned by this task, by design)
- Coverage: This task is README prose editing with no compiler. Verification is (1) the manual-narrative read performed here, and (2) the Task 4-3 tripwire (separate task) that asserts every schema key NAME appears in the README. I confirmed the tripwire's premise holds against the reconciled README: all 19 distinct schema keys appear in backticks — ai_command(8), max_diff_lines(2), timeout(7), diff_exclude(1), context(3), prompt(3), tag_prefix(1), commit_prefix(1), release_branch(1), publish(1), changelog(3), provider(1), on_notes_failure(2), fallback(2), version_file(2), version_pattern(1), preflight(1), pre_tag(2), post_release(3). Container tags (release/commit/hooks) are section headings, not key rows, as the spec requires.
- Confirmation pass on per-key tables vs schema (config.go fileShape/releaseShape/commitShape/hooksShape toml tags): every schema key has a row at the correct level.
  - Shared engine keys (README 201-206): ai_command, timeout, max_diff_lines, diff_exclude — matches the 4 shared decode-shape tags (Release/Commit container tags correctly excluded).
  - [release] (210-225): tag_prefix, commit_prefix, release_branch, publish, changelog, provider, context, prompt, on_notes_failure, fallback, version_file, version_pattern, plus the per-verb ai_command/timeout override rows — matches releaseShape exactly; the dual-level ai_command/timeout rows are intentionally preserved.
  - [release.hooks] (229-233): preflight, pre_tag, post_release — matches hooksShape.
  - [commit] (237-242): context, prompt, plus per-verb ai_command/timeout overrides — matches commitShape.
  - No row missing, none deleted, dual-level rows not collapsed. Tables left intact (correct — the spec says add only a genuinely missing row).
- Notes: I read tests rather than executing them, per task rules. The initgen tests (initgen_test.go) pin the minimal shape and dual-pointer header that the README block mirrors, so the README example is anchored to the binary's actual output via that test.

CODE QUALITY:
- Project conventions: Followed/N/A. Docs-only change; no Go touched. The README block matches initgen.MintTOML() byte-for-intent, honouring the CLAUDE.md "keep comments true to as-built" spirit at the doc level.
- SOLID principles: N/A (prose).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. The intro, the minimal-template block, and the explicit "tables below are the authoritative reference" sentence form a clean, non-contradictory narrative; the Commands init line is concise and consistent.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The acceptance criteria are fully and cleanly met; the README example and the binary output agree byte-for-intent, and every schema key is documented. No do-now/quickfix/idea/bug items rise above the concrete-change floor.
