TASK: 3-1 — Strip MintTOML() to the minimal template (empty body + dual-pointer header) and swap in the minimal-shape tests

ACCEPTANCE CRITERIA (from plan):
- MintTOML() returns a string whose only content is a header comment block — body carries no active `key = value` line and no commented `# key = value` / `# [table]` config line.
- The returned string contains BOTH pointers: a GitHub-docs reference AND the literal `mint setup`.
- initgen.go package-level and MintTOML function-level doc comments describe the as-built minimal template (empty body + dual-pointer header, binary as doc source, header as cold-arrival recovery net); no stale "commented template" / "uncomment to enable" / value-drift-pin claims.
- The twelve commented-template assertions owned by this task are removed (the two value-drift pins were earmarked for 3-2).
- New tests assert the minimal body shape (no active or commented keys) and pin both header pointers.
- ReleaseShim(), ShimMode, shim_test.go byte-for-byte unchanged.
- engine/init_test.go passes unchanged (no edit).
- initgen package compiles after this task.
- Standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Generated config: strip to minimal" (specification.md:56-64): the generated `.mint.toml` is stripped to bare essentials — "Micro-choice (decided): empty body + a short header comment" pointing to the GitHub docs (human config reference) and `mint setup` (AI-assisted setup), the header being the cold-arrival recovery net. "initgen scope of change" (lines 66-70): "Only MintTOML() changes ... ReleaseShim() and its tests are untouched"; the commented-template tests are REMOVED not edited, NEW tests assert minimal shape (empty body + both header pointers). "Definition of done" → "Updated initgen tests" (line 215): scaffold-value drift-pin subsumed by the SoT drift test, ReleaseShim() tests untouched. Aligns exactly with the as-built.

IMPLEMENTATION:
- Status: Implemented (correct; matches acceptance criteria and spec).
- Location: internal/initgen/initgen.go:38-46 (MintTOML body); :1-26 (package doc); :28-37 (function doc).
- Notes:
  - (a) Body is empty: MintTOML() (initgen.go:39-45) returns a raw-string literal that is exactly six `#`-prefixed prose lines plus a blank `#` separator and a trailing newline. No active `key = value` line and no commented config line. CRITERION MET.
  - (b) Both pointers present: `https://github.com/leeovery/mint` (initgen.go:43) satisfies the GitHub-docs reference; the literal `mint setup` (initgen.go:44, wrapped in backticks via string concatenation) satisfies the AI-assisted-setup pointer. CRITERION MET. The header also states the file is fully optional with built-in defaults (initgen.go:39-41), matching the spec's "empty file is valid and honest" requirement.
  - (d) Doc comments are as-built: package doc (initgen.go:1-25) describes "the MINIMAL `.mint.toml` template: an empty body ... preceded by a short header comment", names the binary as the doc source, and frames the header as the "cold-arrival RECOVERY NET". It retains the true statements (PURE generator, no IO, no project auto-detection, no config import, ReleaseShim unaffected). No stale "commented template" / "uncomment to enable" / value-drift-pin literal claims survive (grep confirms `config.` appears nowhere in the file). Function doc (initgen.go:28-37) likewise states the body is EMPTY and pins the contract as "both pointers present and the body carries no key, not the exact header wording". CRITERION MET.
  - (e) ReleaseShim()/ShimMode untouched: shim.go and shim_test.go do not appear in the strip commit's changed-files list (commit 59d527c touched only initgen.go and initgen_test.go among code files); `git show 59d527c -- shim.go shim_test.go init_test.go` returns empty. CRITERION MET.
  - Note on commit sequencing: tasks 3-1 and 3-2 were committed together (commit 59d527c, "Sequenced 3-1+3-2 in one cycle"). The plan's Edge Cases (phase-3-tasks.md:58) explicitly permit this ("an executor may sequence 3-1 then 3-2 in one cycle"). The file state therefore already reflects 3-2's removals too. This is allowed and the package ends green; not a finding.

TESTS:
- Status: Adequate.
- Coverage:
  - TestMintTOML_BodyHasNoActiveOrCommentedKeys (initgen_test.go:16-26) splits the template on `\n` and asserts every line fails the `carriesConfigKey` predicate — proving the body carries no active OR commented config key. This directly pins criterion (a) and the strip's regression net (a reintroduced `# ai_command = ...` line would fail).
  - TestMintTOML_HeaderCarriesBothPointers (initgen_test.go:33-44) asserts the template contains both `github.com` and `mint setup`. Pins criterion (b) / both edges of the recovery-net contract.
  - The `carriesConfigKey` predicate (initgen_test.go:51-73) strips the comment marker, treats `[...]` as a table header, and requires the pre-`=` token to be a single TOML bare key (via `isBareKeyRune`, :77-86). I traced each header line through it: the only lines containing punctuation that could be mistaken for a key are the two URL/`run mint setup` lines, neither of which contains `=`, so `idx <= 0` short-circuits them to false. No header line is mis-flagged; the edge case from the plan (prose/URL `=` not mis-flagged) is handled — though the current header happens to contain no `=` at all, the predicate is still correct against the spec's stricter "prose with `=`" guard.
  - (c) old assertions removed: grep confirms none of the twelve removed test names, nor the now-removed `TestMintTOML_*ValueEqualsConfigConstant` pins (3-2's), nor the orphaned helpers (`activeTopLevelValue`, `isConfigLineForKey`, `valueAfterEquals`, `commonKeyDefaults`, `optionalKeys`, `looksLikeConfigLine`) remain. CRITERION MET.
  - (e) shim tests untouched and (g) engine/init_test.go content-agnostic: init_test.go compares written `.mint.toml` against `initgen.MintTOML()` (init_test.go:66, 153, 225) and pins root entries as exactly `[".mint.toml","release"]` (:325), so the strip flows through with no edit. Untouched in the commit.
- Notes:
  - Not under-tested: both acceptance criteria for this task (empty body, both pointers) have a dedicated, failing-if-broken test. The body-emptiness test iterates ALL lines, so it would catch a regression anywhere in the template.
  - Not over-tested: two focused tests plus two small helpers; no redundant assertions. The helpers are referenced by the surviving test (`carriesConfigKey` → `isBareKeyRune`), so no orphan. Import block is minimal (`strings`, `testing`, `mint/internal/initgen` only) — `config`/`time`/`strconv`/`bufio`/`os`/`path/filepath` all severed, confirming 3-2's import contract and the initgen↛config seam.

CODE QUALITY:
- Project conventions: Followed. External test package (`package initgen_test`), `t.Parallel()` on both tests, table-free but shape-appropriate. Pure generator, no IO, no config import — matches CLAUDE.md's "initgen is the pure mint init template generator" and "deliberately does NOT import config". Comments are true-to-as-built per CLAUDE.md's comments rule.
- SOLID principles: Good. Single-responsibility generator; the test predicate is small and self-contained.
- Complexity: Low. MintTOML is a single return; the predicate is a short linear scan.
- Modern idioms: Yes. Raw-string literal with backtick-escaping via concatenation for the embedded `mint setup` backticks (initgen.go:44) is the idiomatic Go approach.
- Readability: Good. Header prose is clear and honest; test doc comments explain the recovery-net contract and why a header URL/`=` is exempt.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/initgen/initgen.go:43 — the header docs URL is `https://github.com/leeovery/mint` (the repo root), not a dedicated docs/README path. The plan (phase-3-tasks.md:70) flags cross-phase consistency with the Phase 4 README entry point: if Phase 4 settles on a different canonical docs URL, align this header. The test pins presence (`github.com` substring) not the exact URL, so the surfaces can converge without a brittle pin — decision deferred to Phase 4, no change required now.
- [idea] internal/initgen/initgen_test.go:38 — TestMintTOML_HeaderCarriesBothPointers asserts only a bare `github.com` substring. This is intentionally loose per the plan (presence-not-wording), but it would also pass on any incidental `github.com` mention. If Phase 4 fixes a canonical docs token, consider tightening to that token. Decide during Phase 4 README work; not a defect now.
