# Consolidation Tasks: Token Aware Diff Sizing (Phase 3)

## Task 1: Regenerate refuses an unusable notes file before it resolves the publisher
placement: phase 3
severity: behaviour

**Problem**: `mint release regenerate <version> --notes-file P` resolves the version tag and the provider in the cmd layer (`cmd/mint/main.go:205`, `:224` → `ResolvePublisher` → `warnPublishDowngraded`) before `RegenerateRun` reads P (`internal/engine/regenerate_interactive.go:164`). With a non-github origin or no remote, a missing, empty or `""` P prints `publish skipped: provider could not be resolved (...); downgrading to tag + push only` ahead of the refusal. Any other provider-resolution error aborts in place of the refusal, so the operator fixes the provider first and learns only on the next run that P is unusable. §13.2 and §13.4 put the read at startup, before any check, and the refusal before any step. Release keeps that order (`internal/engine/release.go:319-323`); regenerate does not, and `internal/engine/notesfile_refusal_test.go` drives `RegenerateRun` directly, so the cmd-layer order is untested.
**Solution**: Read P at the head of `runRegenerateSingle`, ahead of version-tag and publisher resolution, through an engine entry point that wraps `readNotesFile` and surfaces its refusal on the presenter at stage `notes`. Carry the read body into `RegenerateRunRequest` so `RegenerateRun` no longer reads the file; `NotesFile` stays for `notesExclusion`. Derived from §13.2's read-at-startup rule and release's existing order: reading first is the one shape that puts the refusal ahead of every check, where moving only the publisher call would still leave the tag resolution in front. Add a cmd-level test: with an unresolvable origin and an empty P, the output holds the refusal and no `publish skipped` warning.
**Outcome**: On both verbs, an unusable `--notes-file` is the first and only thing the run reports.
**Acceptance Criteria**:
- [ ] `mint release regenerate 1.4.0 --notes-file P` with no `origin` remote or a non-github one, and P empty, whitespace-only, missing or `""`, reports the one `notes` refusal (`--notes-file P is empty`, or `reading --notes-file P: <cause>`), shows no `publish skipped` warning, runs neither the tag lookup (`git tag --list`) nor the remote read (`git remote get-url origin`), and exits 1 (§13.2, §13.4)
- [ ] `mint release regenerate 9.9.9 --notes-file P` with P unusable and no `v9.9.9` tag reports the `notes` refusal, not `no tag v9.9.9 found`, and exits 1, not 2 (§13.4)
- [ ] With a usable P given as a relative path, regenerate resolves it against the directory mint was run from, asks no source question, and ships the body read at startup to the chosen targets byte-identical, even when P is rewritten later in the run (§13.1, §13.2, §13.3)
- [ ] Without `--notes-file`, regenerate reads no file and runs as before: the publisher resolves, a non-github origin still warns `publish skipped`, and the source question is asked (§13.1)
**Do**:
- `cmd/mint/main.go` `runRegenerateSingle` (`:204`): the notes-file read is its first step, ahead of `version.ResolveRegenerateTarget` (`:205`) and `resolveRegeneratePublisher` (`:224`); a refusal returns through `exitCode`, which maps the `*engine.AbortError` to 1
- `internal/engine`: an exported entry point wrapping `readNotesFile` (`internal/engine/release.go:901`) that surfaces its error on `deps.Presenter` at stage `notes`, the stage and messages `Release` already uses at `internal/engine/release.go:319-323`
- `RegenerateRunRequest` (`internal/engine/regenerate_interactive.go:105`) carries the read body beside `NotesFile` (the findings' shape: a `Supplied *string`); `RegenerateRun` drops its read at `:164-167` and takes the body from the request. `NotesFile` stays, since `notesExclusion` (`:204`) still needs the path
- The engine tests that hand `RegenerateRun` a path to read move onto the read body: `notesFileRunReq` (`internal/engine/regenerate_notesfile_test.go:31`) and every test built on it, and `TestRegenerateRun_UnusableNotesFile_IsRefusedBeforeAnyStep` (`internal/engine/notesfile_refusal_test.go:93`), whose refusal cases now belong to the new entry point
- The cmd-level test drives `runRegenerateSingle`, which takes its deps and runner as parameters, with a `FakeRunner` and a `RecordingPresenter`; `regenDeps` (`cmd/mint/regenerate_publisher_test.go:22`) already builds that pair

## Task 2: The tag annotation carries the notes body byte-identical
placement: phase 3
severity: behaviour

**Problem**: `createAnnotatedTag` (`internal/release/release.go:99`) runs `git tag -a <tag> -F -` with no `--cleanup`. git's default for `git tag` is `strip` (`man git-tag`, git 2.56.0), which removes whitespace and every line starting with `#`. Hand-written Markdown notes, the common case `--notes-file` opens, lose their `## …` headings, `#123` lines, two-space line breaks and repeated blank lines from the tag annotation without any message. The changelog and the provider release keep them verbatim. The operator approved the full body at the review gate, so the first sign is a tag body that differs from the GitHub release, and a later `regenerate --source tag` copies the stripped body onto other surfaces. This breaks README.md:107 ("byte-identical"), §13.3 and CLAUDE.md's byte-identical-bodies invariant. The phase's own fixture (`internal/engine/release_notesfile_test.go:22-25`, `"## ✨ Added\n"` and a trailing `"  \n"`) has exactly this shape and passes only because FakeRunner never runs git's cleanup.
**Solution**: Add `--cleanup=verbatim` to the annotated-tag argv in `createAnnotatedTag`, so the annotation is `composeTagMessage`'s bytes on every notes route. Derived from the byte-identical invariant: verbatim is the only cleanup mode that changes nothing. Update the pinned tag argv the findings list (`internal/release/release_test.go:40`, `internal/engine/release_test.go:387`, `:1540`, `internal/engine/release_commitgraph_test.go:33`, `internal/engine/release_downgrade_test.go:62`, `internal/engine/release_setversion_test.go:177,181,224`, `internal/engine/release_priortag_test.go:390`). Check that the tag-body reader behind `regenerate --source tag` (`ReadTagBody`/`RequireTagBody`, `internal/engine/regenerate_tagsource.go`) still splits subject from body once trailing whitespace is no longer normalised.
**Outcome**: The tag annotation carries the reviewed notes body byte-for-byte, the same bytes as the changelog section and the provider release, whatever the body holds.
**Acceptance Criteria**:
- [ ] A release whose notes body carries `## ` headings, a line beginning `#123`, a line ending in two spaces and repeated blank lines creates its annotated tag with `--cleanup=verbatim` and pipes `{commit_prefix} Release {tag}`, a blank line, then that body byte-for-byte; the tag body, the changelog section and the provider release body are the same bytes (§13.3)
- [ ] No notes route creates the tag without `--cleanup=verbatim`: an AI body, a body edited at the review gate, a `--notes-file` body, the `--no-ai` commit-subject list, and the first-release and empty-diff bodies all go through the same tag argv (§7.1, §13.3)
- [ ] Apart from the added flag the tag step is unchanged: an annotated tag named `<tag>` with its message read from stdin (`-F -`), followed by `git push --atomic origin HEAD <tag>`
**Do**:
- `internal/release/release.go:99` `createAnnotatedTag`: the argv gains `--cleanup=verbatim`; `composeTagMessage` (`:124-127`) is unchanged
- Update every pinned tag argv. Measured with `rg -n '"tag", "-a"' -g '*_test.go' internal cmd` (13 hits in 9 files, of which `internal/git/mutator_test.go:319`, `:355` and `internal/runner/fake_runner_test.go:143` exercise the Mutator and the FakeRunner with a tag-shaped argv of their own and stay as they are) and `rg -n '"git tag -a [^"]*"' internal cmd` (2 hits): 12 sites, all updated —
  - `internal/release/release_test.go:40`
  - `internal/engine/release_test.go:151`, `:387`, `:1540`
  - `internal/engine/release_provider_test.go:118`
  - `internal/engine/release_versionfile_test.go:208`
  - `internal/engine/release_commitgraph_test.go:33`
  - `internal/engine/release_downgrade_test.go:62`
  - `internal/engine/release_setversion_test.go:177`, `:181`, `:224`
  - `internal/engine/release_priortag_test.go:390`
  The Solution's list leaves out `release_test.go:151`, `release_provider_test.go:118` and `release_versionfile_test.go:208`. `release_setversion_test.go:181` asserts the `v1.2.4` tag was not created; left on the old argv it would pass vacuously
- Three helpers match the tag by the prefix `git tag -a` (`internal/engine/release_test.go:1311`, `internal/engine/release_preflighthook_test.go:245`, `internal/engine/release_pretaghook_test.go:177`); they change only if the new argv no longer begins `git tag -a`
- Check that the tag-body reader behind `regenerate --source tag` (`ReadTagBody`/`RequireTagBody`, `internal/engine/regenerate_tagsource.go:41-66`, which returns `git for-each-ref --format=%(contents:body)` stdout raw and trims only for its has-body decision) still splits subject from body once trailing whitespace is no longer normalised
