AGENT: duplication
FINDINGS:
- FINDING: Three near-identical atomic-write helpers (temp-file + write + close + rename, with cleanup-on-error)
  SEVERITY: medium
  FILES: internal/record/changelog.go:219, internal/record/versionfile.go:203, internal/notescache/cache.go:211
  DESCRIPTION: Three independently-authored implementations of the same crash-safe write idiom — os.CreateTemp in the target dir, WriteString/Write, Close (with os.Remove cleanup on each error branch), optional Chmod(0o644), then os.Rename onto the final path. record's two (writeAtomic, writeFileAtomic) are in the SAME package and identical in shape apart from the temp prefix and the error-message nouns ("changelog" vs "version file"); versionfile.go even carries the comment "It mirrors writeAtomic ... but keeps its own temp-file naming." notescache's copy is the same algorithm minus Chmod and with bare (un-wrapped) errors. Rule-of-Three: three copies of a ~25-line crash-safety primitive that must stay correct in lockstep — a fix to temp-cleanup or rename ordering in one will silently not reach the others.
  RECOMMENDATION: Extract ONE shared helper — e.g. an internal atomicwrite/fsutil package exposing WriteFile(path string, data []byte, perm fs.FileMode) error doing CreateTemp(dir)/Write/Close/Chmod/Rename with the established cleanup branches. Have all three sites delegate, passing their perm (0o644) and wrapping the returned error with their domain noun AT THE CALL SITE so per-domain context is preserved. The two record copies are the highest-value merge since they already share a package and diverge only cosmetically.

- FINDING: `git remote get-url origin` reader duplicated verbatim across the cmd/engine boundary
  SEVERITY: low
  FILES: cmd/mint/main.go:191 (regenerateRemoteURL), internal/engine/release.go:861 (remoteURL)
  DESCRIPTION: regenerateRemoteURL (cmd) and remoteURL (engine) are line-for-line identical: r.Run(ctx, "git", "remote", "get-url", "origin"), return "" on any non-zero exit, otherwise TrimSpace(stdout). Both feed publish.ResolvePublisher with the same "empty == unresolved, downgrade rather than fail" contract; the cmd copy's comment even says it does this "the same way the forward path does." A future change to how the release remote is named would have to be made twice.
  RECOMMENDATION: Export the engine's reader (e.g. engine.RemoteURL(ctx, r)), have runRegenerateSingle call it, and delete regenerateRemoteURL.

- FINDING: Cross-package date-layout constant "2006-01-02" redefined instead of shared
  SEVERITY: low
  FILES: internal/record/changelog.go:19 (dateLayout), internal/engine/regenerate_write.go:119 (regenerateDateLayout)
  DESCRIPTION: The changelog section-header date layout is defined twice as the same literal. record OWNS the layout (it formats every ## [x.y.z] - <date> header); engine redefines it solely to PARSE a historical date back into the SAME format record re-emits — its comment says it exists so "the healed header matches existing sections exactly." They must stay equal by construction (a mismatched parse layout silently breaks the no-empty-commit / no-data-loss guarantees), yet nothing enforces it; coupled by a copied literal.
  RECOMMENDATION: Have record export the canonical layout (e.g. record.ChangelogDateLayout) and have engine/regenerate_write.go parse with it, removing the engine-local constant. engine already depends on record for the write itself.

- FINDING: changelogFileName = "CHANGELOG.md" constant duplicated across engine and record
  SEVERITY: low
  FILES: internal/engine/regenerate_changelog.go:41, internal/record/changelog.go:16
  DESCRIPTION: The fixed repo-root changelog name is a package-level const in both record (the owner — it writes the file) and engine (which stages it via git -C {root} add CHANGELOG.md in both regenerate_changelog.go and regenerate_batch_changelog.go). The engine copy's comment acknowledges the mirror. The staged path MUST equal the written path, but the equality rests on the literal being copied correctly rather than on a shared symbol. (notes/assemble.go independently hard-codes the same name inside :(exclude)CHANGELOG.md — related but distinct.)
  RECOMMENDATION: Export record.ChangelogFileName (record owns the file) and have the engine's two staging sites reference it, deleting the engine-local const — making the "written == staged" invariant a compile-time fact.

- FINDING: Parallel changelog target/changelog-disabled validators carry a copied error literal
  SEVERITY: low
  FILES: cmd/mint/regenerate_validate.go:57 (validateTargetAgainstChangelog), internal/engine/regenerate_batch.go:118 (checkBatchTargetConfig)
  DESCRIPTION: Both implement the identical rule (changelog-touching target with changelog disabled is rejected) and return the identical literal "changelog is disabled in config". The two FUNCTIONS are an intentional cross-boundary mirror (different concrete enums — cmd's regenerateTarget vs engine's RegenerateTarget; the engine enforces independently so a direct engine caller cannot bypass), so merging the functions is NOT warranted. What drifts is the spec-pinned error MESSAGE, maintained separately in two literals.
  RECOMMENDATION: Keep both validators (the parallelism and concrete-enum split are correct), but lift only the shared message into a single exported sentinel (e.g. engine.ErrChangelogDisabled) both reference, so the pinned wording cannot diverge. Do NOT merge the functions or thread untyped values across the boundary.
SUMMARY: The codebase is broadly well-factored (config defaults, version resolution, notes exclusion, source/target axis translation already consolidated) and most cross-boundary parallels are deliberate. The one genuinely high-value extraction is the triplicated atomic-write helper (two copies in record, one in notescache); the remaining four are small shared-constant/shared-reader consolidations.
