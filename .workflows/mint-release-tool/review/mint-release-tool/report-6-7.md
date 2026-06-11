TASK: mint-release-tool-6-7 — mint init command (drops both files, idempotent/non-clobbering, --force)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. Engine orchestrator internal/engine/init.go:111-165 (Init, scaffoldTarget, fileExists); CLI cmd/mint/init.go:36-74 (parseInitFlags, runInit); dispatch main.go:63-64,266-268. Root via gitrepo.ResolveRoot. Content from initgen.MintTOML()/ReleaseShim()/ShimMode (0o755). Two independent targets; existence via fileExists; non-clobber default skips w/ "exists, use --force"; --force overwrites + reports InitCreated. Explicit os.Chmod after os.WriteFile defends overwrite-keeps-old-perms + umask. Root-resolution failure aborts before any target. Init calls neither Prompt nor RunFinished.

TESTS:
- Status: Adequate. init_test.go covers every AC 1:1 — neither-exists creates both w/ exact content, shim executable mode (+ &0o111 check), only-config/only-shim independence (existing unchanged), both-exist skipped no-overwrite, --force rewrites both as InitCreated, --force restores executable mode, writes at git-resolved root (one show-toplevel invocation), exactly two entries (no hook/prompt dir), no RunFinished/no Prompt, root-resolution failure aborts. init_flags_test.go + dispatch_test.go + presenter/init_test.go.

CODE QUALITY:
- Followed conventions (seam-based DI w/ FakeRunner + RecordingPresenter, table-free focused tests, t.Parallel, doc comments). SOLID good — InitDeps narrower than ReleaseDeps (ISP); engine-orchestration/initgen-content/presenter-rendering split. Low complexity, intent-rich comments (Chmod footgun).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/init.go:162-165 — fileExists treats any non-IsNotExist stat error (e.g. permission failure) as "exists", silently skipping the target rather than surfacing the real error. Direction (err toward not-clobbering) is documented and defensible, but whether a genuine stat failure should skip-silently vs abort with a diagnostic is a design decision worth confirming; no test exercises a stat error other than not-exist.
