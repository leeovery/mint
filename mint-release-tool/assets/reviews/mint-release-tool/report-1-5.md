TASK: mint-release-tool-1-5 — Local preflight gates (clean tree, on branch, tag-free local)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented
- Location: internal/preflight/preflight.go (CheckCleanTree, CheckOnBranch, CheckTagFreeLocal, RunLocalGates; GateError/newGateError)
- Notes: CheckCleanTree runs `git status --porcelain`, never passes --ignored (gitignored exempt by construction), wraps real git errors. CheckOnBranch runs `git rev-parse --abbrev-ref HEAD`; mismatch GateError names both branches. CheckTagFreeLocal runs `git rev-parse -q --verify refs/tags/{tag}`, distinguishes clean non-zero (tag absent → pass) from ErrCommandNotFound (hard error). RunLocalGates orders cheap-first, aborts on first failure. As-built drift (not a defect): file also holds network/gh gates and the Phase-4 anyBranch param, legitimately wired end-to-end by later phases; with anyBranch=false the 1-5 contract holds exactly.

TESTS:
- Status: Adequate
- Coverage: empty→pass with exact argv; dirty table (modified/staged/untracked); on-branch match + differs-naming-both; tag absent/exists; tag-check ErrCommandNotFound is hard error; RunLocalGates all-pass + cheap-first abort. Gitignored-only covered behaviourally (empty porcelain + no --ignored argv).

CODE QUALITY:
- Followed conventions (%w wrapping, GateError via errors.As, black-box tests, t.Parallel, mock the seam). SOLID good, low complexity, exemplary readability (comments state exact git command + rule).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/preflight/preflight.go:96 — CheckCleanTree's abort message is fixed and doesn't surface what is dirty (unlike sibling gates naming branch/tag); decide whether to include a short summary of porcelain output (first N paths or file count) so the abort is equally actionable.
- [quickfix] internal/preflight/preflight_test.go:18 — the gitignored-exempt criterion is proven only implicitly; add one focused subtest named for the gitignored case (clean tree, gitignored present → porcelain empty → passes, asserting no --ignored arg) so the criterion is pinned by name.
