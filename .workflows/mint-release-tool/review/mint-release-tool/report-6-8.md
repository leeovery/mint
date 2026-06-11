TASK: mint-release-tool-6-8 — mint version / mint --version

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. Single source internal/buildinfo/buildinfo.go:19 (var Version = "dev", ldflags-injectable). Entry points cmd/mint/version.go (hasVersionFlag, runVersion, emitVersion); routing main.go:50-51 (--version flag, before classification) + :65-66 (version verb). Both branches call same runVersion → emitVersion → single p.ShowVersion(Version{Value: buildinfo.Version}). Plain render plain.go:332-334 ("%s\n", bare value, stdout only); pretty render pretty.go:712-716 ({leaf} mint v{value}, dim, empty leaf → 🌿). No runner/repo-root in the version path; no Prompt/gate, no RunFinished. Byte-identity structural (both run branches funnel through one runVersion/emitVersion/ShowVersion).

TESTS:
- Status: Adequate (one tautological test noted). version_test.go: single ShowVersion call + nothing else, value from pinned buildinfo.Version, plain bare-value+newline $(...) contract, empty Leaf, hasVersionFlag detection, runs outside a git repo (exit 0). buildinfo_test.go: "dev" default + var-overridable. dispatch_test.go: version verb routing. presenter/version_test.go: render contract.

CODE QUALITY:
- Followed conventions (single-source var w/ build-time injection, thin main/run shell, presenter seam, no git in version path). SOLID good — emitVersion single shared core. Low complexity, %s formatting, precise comments about $(mint version) contract.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/version_test.go:72-82 — TestSubcommandAndFlagAreByteIdentical is tautological: both halves call the identical emitVersion(presenter.New(ModePlain,...)), asserting determinism not surface-identity. Make it actually exercise the two entry points — drive run([]string{"version"}) and run([]string{"--version"}) (capturing stdout) and assert bytes equal — so it fails if the two run() branches ever diverge.
- [quickfix] cmd/mint/main.go:50-66 — neither run() version branch has an end-to-end test through run; only runVersion in isolation + classifyCommand are tested. Add a test calling run([]string{"--version"}) and run([]string{"version"}) asserting exit 0 (+ captured bare-value output), closing the routing gap.
- [do-now] cmd/mint/version.go:37 — the runVersion doc comment says the presenter is "constructed non-interactively (yes=true)"; the call is NewForStartup(false, true, ...) where the second arg is yes. A brief inline note mapping false→plainFlag (so TTY detection still governs plain/pretty for mint --version on a terminal) would prevent a future misread that version forces plain.
