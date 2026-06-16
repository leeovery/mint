# Plan: Interactive Mint Init Setup

## Phases

### Phase 1: Config-metadata SoT + drift test
status: draft

**Goal**: Establish the single in-binary, schema-adjacent config-metadata table — one row per (level, key) pair carrying key · level · default · description — and the drift test that mechanically derives the schema's leaf-key set from the config decode-shape structs and proves a total bijection against it.

**Why this order**: This table is the foundation every downstream surface renders from (the `mint setup` config reference) and the carrier into which the old `initgen` scaffold value-drift pin is subsumed. Building it first, with anti-drift enforcement in place, gives the strongest foundation before anything consumes or renders it — no forward reference to a render target that doesn't yet exist.

**Acceptance**:
- [ ] A structured SoT of config metadata lives in the binary with one row per config key, carrying the four columns key · level · default · description.
- [ ] `ai_command` and `timeout` appear as distinct rows at the shared top level and under `[release]` and `[commit]`; the `release`/`commit`/`hooks` container fields emit no row and are recursed into to supply the level for their leaf rows; `[release.hooks]` keys are their own rows.
- [ ] The `default` column uses the decided representation convention: empty-string defaults blank, sentinel-auto defaults as `auto`, empty collection as `[]`, per-verb inherit defaults as `shared`, and hooks keys with no default blank/`—`.
- [ ] A Go drift test derives the authoritative leaf-key set mechanically from the `fileShape`/`releaseShape`/`commitShape`/`hooksShape` `toml` tags (not a hand-maintained list) and fails the build on any divergence — a schema leaf with no matching SoT row at its level, or an SoT row with no matching leaf.
- [ ] The default values previously pinned by `initgen` (`DefaultAICommand`, `DefaultTimeout`) are carried and pinned by the SoT `default` column, so no default value is left unpinned.
- [ ] All standard gates pass (`go build`, `gofmt -l`, `go vet`, `go test -race`, `golangci-lint`).

### Phase 2: mint setup subcommand — guide emitter, sections, help wiring
status: draft

**Goal**: Add the new top-level `mint setup` verb as a pure stdout string emitter (in the spirit of `initgen`) that prints the embedded AI setup guide — the inspect-and-map procedure, AI etiquette, minimalism rule, and the existing-config/upgrade branch — plus the config reference rendered from the Phase 1 SoT, each required section carrying a stable test-detectable marker, running unconditionally with no git/cwd guard, and threaded through mint's curated-help surface.

**Why this order**: `mint setup` is the core new user-facing capability and the SoT's single in-binary render target. It consumes the Phase 1 table and integrates with the existing `cmd/mint` dispatch and curated-help patterns. It must exist before `initgen` can honestly point its header at it.

**Acceptance**:
- [ ] `mint setup` emits the guide to stdout and performs no IO beyond stdout; it runs unconditionally (prints even outside a work tree — no `git rev-parse` / cwd guard).
- [ ] The emitted output renders the config reference (key · level · default · description table) from the Phase 1 SoT.
- [ ] Each required section emits a stable section marker; a structural test greps the markers — pipeline/hook model, etiquette, minimalism, the existing-config/upgrade branch, and the config reference — and proves each section's presence without coupling to body prose.
- [ ] `classifyCommand`/`run` route a new `commandSetup`; `rootUsage` gains a curated one-line `setup` command description; a curated `setupUsage` exists and `mint setup --help` prints it to stdout with exit 0 via the `flag.ErrHelp` path.
- [ ] The existing usage-coverage test is extended to pin `mint setup` (rootUsage line + `setupUsage`); `mint help` stays the frozen curated text apart from the added `setup` line and carries no config reference.
- [ ] All standard gates pass.

### Phase 3: initgen strip-to-minimal
status: draft

**Goal**: Change `MintTOML()` to return the minimal `.mint.toml` — an empty body plus a header comment pointing to the GitHub docs and `mint setup` — remove the commented-template test assertions, and add tests pinning the minimal shape and both header pointers.

**Why this order**: Stripping the in-repo config docs is only honest once `mint setup` (Phase 2) exists to carry the agent's config reference and the header can point to it; the header pointer is also the cold-arrival recovery net. This depends on Phase 2 and is isolated to `MintTOML()` — `ReleaseShim()` and `mint init` behaviour are untouched.

**Acceptance**:
- [ ] `MintTOML()` returns an empty-body file with a header comment carrying both pointers — the GitHub docs (human config reference) and `mint setup` (AI-assisted setup).
- [ ] The commented-template assertions are removed (active-keys-at-defaults, optional-keys-present-but-commented, per-verb override comments, `pre_tag` both forms, prompt-override-in-comment-only, hooks-only-under-`[release.hooks]`, uncommented-loads-cleanly).
- [ ] New `initgen` tests assert the minimal shape — empty body with no active or commented keys — and pin the header's two pointers as the load-bearing recovery net.
- [ ] `ReleaseShim()` and its tests are unchanged; `mint init` still emits both `.mint.toml` (now minimal) and the `release` shim at the git-resolved repo root with unchanged non-clobber / `--force` / idempotent behaviour.
- [ ] All standard gates pass.

### Phase 4: README reconciliation + entry-point + tripwire
status: draft

**Goal**: Make the README the human config-reference surface and the entry point that routes operators to `mint setup` — correct the now-false Configuration and Commands framing, replace or drop the embedded full commented-template block, add the tiny "any AI" entry-point prompt, verify the per-key tables declare every config key and its default, and optionally add the key-presence tripwire test.

**Why this order**: The README routes to `mint setup` (Phase 2) and must reflect the now-minimal template (Phase 3), so it is the final human-facing layer once both binary surfaces are complete. It adds no scope to the binary — it reconciles documentation with what the prior phases shipped.

**Acceptance**:
- [ ] The `## Configuration` intro and the Commands-section `init` line state that `mint init` now writes a minimal `.mint.toml` (empty body + header pointer), correcting the "commented `.mint.toml`" framing.
- [ ] The embedded full commented-template TOML block is replaced with the new minimal template (empty body + header) or dropped, leaving the per-key reference tables as the authoritative human config reference.
- [ ] The README carries the entry-point prompt routing operators to `mint setup` ("run `mint setup` and follow what it prints"), framed for any AI with the light Opus-level steer and no fidelity-floor machinery.
- [ ] The per-key reference tables are verified to declare every config key and its default; an optional tripwire test asserts every schema key name appears somewhere in the README.
- [ ] All standard gates pass.

## Manual Acceptance (not a phase)

The spec's "Acceptance — prose quality" is a one-time manual run against representative repos (a fresh JS project, a Go project, a repo with an existing release script, a repo with an existing `.mint.toml`), eyeballing that each yields a sensible config. It cannot be unit-tested (spawning an AI is forbidden by the test culture). Tracked as a manual acceptance step after Phases 2 and 4 land — not modelled as implementation work.
