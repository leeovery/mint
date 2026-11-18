TASK: mint-release-tool-2-16 — End-to-end prior-tag release wiring

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go threads full flow: Stage 4 resolveBody builds Assembler+Generator+Selector from deps.Runner and runs SelectBodyWithReuse over SelectState (diff base LastTag..HEAD). Gate via reviewGate, variant by gateForKind (KindNormalAI→y/n/e/r, others→y/n/e). Body flows whole to tag/CHANGELOG/provider. Gate-no→Unwind; pre-push→surfaceAndUnwind; post-push publish failure→warnPublishFailed only. -y never branches around Prompt. Production leaves Transport/Regenerator nil; aiTransport + perRunRegenerator build real seams.

TESTS:
- Status: Adequate. release_priortag_test.go — 12 focused tests; Transport left nil so REAL ai.Transport driven over FakeRunner (claude scripted). Covers all-three-sinks identity + priorTag..HEAD base, diff_exclude globs, plain/embedded version_file exclude, event protocol + r offered, gate-accept→Record/tag/push/publish, gate-abort clean, -y Prompt fires once, regen end-to-end, --no-ai fallback git log, degenerate StubBody no-claude, publish-fails-after-push warns only, notes-failure abort before mutation. Body identity byte-for-byte across sinks.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, single runner/Mutator, read-only vs mutation split). SOLID/DRY good — unified selector collapses four notes paths; RemoteURL shared. Acceptable complexity (Release long but linear/commented), good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
