# CLAUDE.md

Mint is a Go CLI that cuts releases (`mint release`), regenerates release notes (`mint release regenerate`), mints AI-generated commits (`mint commit`), scaffolds a repo (`mint init`), and emits an AI-agent setup guide (`mint setup`). See README.md for the user-facing surface. This file is the working contract for changing the code.

## Gates

Every change must pass all of these before it is done:

```bash
go build ./...
gofmt -l .            # must print nothing
go vet ./...
go test -race ./...
golangci-lint run     # must report 0 issues
```

## Layout

- `cmd/mint` — thin entry: flag parsing (stdlib `flag`, per verb), seam construction, dispatch, exit-code mapping. No business logic.
- `internal/engine` — the release/regenerate/init orchestrators (the release spine, surgical unwind, hook wiring, the notes cache real and dry runs share, the notes floor and its next moves, the chunk-notes keeper).
- `internal/commit` — the `mint commit` orchestrator (gate loop, editor fallback, deferred staging, push).
- `internal/presenter` — the ONLY output surface: pretty + plain renderers, gates, prompts. `presentertest.RecordingPresenter` is the test double.
- `internal/runner` — the ONLY subprocess surface: `CommandRunner` (`Run`/`RunWith`/`RunInDir`/`RunInteractive`). `FakeRunner` is the test double.
- `internal/git` — `Mutator`, the lock-resilient git mutation sink, and `Exclusion`, the root-anchored literal pathspecs that keep a `--notes-file` out of tree checks and commits. `internal/gitrepo` — read-side repo resolution.
- `internal/ai` — content-agnostic AI transport (finished prompt in, body out). `internal/aitransport` — the ONE place mapping a resolved config + verb to a production `ai.Transport` (the three wiring sites — release, regenerate, commit — consume it). `internal/notes` — the release-notes engine: context assembly (diff + Change Map), the body `Selector` (precedence: `--notes-file`, first release, empty diff, `--no-ai`, then AI), single-pass or chunked generation with one merge call, SIZE classification and halving, the exclusion check, the saved chunk-notes body, and failure causes (`ClassifyFailure`/`ResolveFailure`; size sentinels in `size.go`). It exposes `ErrDiffTooLarge`, the over-cap sentinel commit's size guard wraps.
- `internal/config` — the canonical `.mint.toml` schema, strict decoding, and the config-metadata SoT (`MetadataRows()`: one row per `(level, key)` carrying key·level·default·description, drift-tested against the decode-shape structs), and `AppendReleaseGlobs`, the byte-preserving `.mint.toml` writer the exclusion check's answers go through. `internal/initgen` — the pure `mint init` generator (a minimal empty-body + dual-pointer header template, plus `ReleaseShim()`).
- `internal/setupguide` — the embedded `mint setup` guide emitter: renders the SoT config reference plus the marker-anchored guide sections (pipeline/hook model, etiquette, minimalism, existing-config/upgrade). `internal/configtest` — test-only seam indexing the SoT by `(level, key)` (never linked into the binary).
- Supporting: `internal/version` (SemVer), `internal/publish` (provider drivers), `internal/record` (bookkeeping commit), `internal/hooks`, `internal/notescache`, `internal/preflight`, `internal/release`, `internal/buildinfo`, `internal/fsutil`, `internal/cmdline` (POSIX-quoting `ai_command` split, no shell; `Quote` for the paste-able commands mint prints), `internal/bytesize` (the decimal size format narration and the exclusion check share), `internal/runclose` (each run's one close: its outcome, end station and time, and the work deferred past it).
- `demo/` — the VHS tapes that record the README's art: real `mint` commands typed into a terminal against a world of its own (`demo/world.sh`: the `pigeon` repo, a local origin, and shims for the AI, `gh`, `git` and the build). A change to pretty output records its verb's tapes (`demo/record.sh <name>`) and checks the frames by eye before review. `demo/README.md` has the setup and the gotchas.

## Non-negotiable seams

1. **Every external command goes through `runner.CommandRunner`.** Never `os/exec` directly. `RunInteractive` is ONLY for handing the terminal to a child (the user's `$EDITOR`). Read-side git that uses cwd-relative pathspecs (`-- .`, `ls-files`) must run at the repo root via `RunInDir(root, …)` — a plain `Run` from a subdirectory silently scopes to the subtree.
2. **Every git MUTATION goes through `git.Mutator`** (`Mutate(ctx, stdin []byte, name, args...)`) so it is lock-resilient. Reads stay on the plain runner. Bodies are passed as `[]byte`, never a shared `io.Reader` (a retry must re-pipe the full payload).
3. **Every byte of user output goes through `presenter.Presenter`.** No `fmt.Print`/`os.Stdout`/`os.Stderr` in business logic (the cmd layer's usage/error lines and `mint setup`'s guide text are the only exceptions). Events are payload-driven — the presenter never re-derives or invents narration. `NewForStartup(plainFlag, yes, stdout, stderr, stdin)` is the single production construction site; TTY detection never happens downstream.
4. **Gates:** business logic answers a gate only through `Prompt(gate)` — `-y` auto-accept (and its echo) happens INSIDE the presenter, never as an engine-side branch. Whether an unattended run (`-y` or non-TTY stdin) is offered a gate at all is the engine's decision, from the startup `Yes`/`StdinInteractive` signals threaded into its deps: the notes floor stops an unattended run with next moves instead of asking, and the exclusion check prints its suggestions instead of pausing. `ErrNotInteractive` is already rendered by the presenter (wrap it, add no narration); `ErrInputClosed` is NOT rendered (the caller surfaces it). Free-text reads go through `AskLine` — the engine never reads stdin.
5. **AI goes through `internal/ai`** behind a consumer-defined one-method interface at the point of use (see `notes/generate.go` and `commit/generate.go`). Production transports are built in exactly one place — `aitransport.New(runner, cfg, verb)` — which sources BOTH the command and the per-attempt timeout from the per-verb config accessors (never zero-by-omission); the transport stays content-agnostic and never imports `config`. The transport owns validation and the single bad-content retry — consumers never re-retry (the notes generator's SIZE halving re-cuts a slice the model rejected as too big into new, smaller calls; that is not a retry of the same call). The notes `Transport` must be safe for concurrent use: up to four chunk calls run at once. Sentinels: `ErrGenerationFailed` / `ErrTimeout` / `ErrCommandMissing`; `context.Canceled` propagates unchanged (a cancel is not an AI failure — never route it to a fallback).
6. **Config is strict.** All keys live in `internal/config`'s canonical schema; `DisallowUnknownFields` rejects unknowns; new keys need a `typeErrorMessages` entry for a friendly type error. `ai_command` and `timeout` live at BOTH levels — a top-level shared baseline plus an optional per-verb override — resolved per-key `[verb] → shared → shipped default` by the typed `AICommandFor`/`TimeoutFor` accessors (keyed on the closed `Verb` enum, which has no `regenerate` value — regenerate rides on `[release]`); `max_prompt_bytes` is shared-only at the top level. `diff_exclude` has shared, `[release]` and `[commit]` levels whose lists ADD rather than override: the release-notes diff (forward release and regenerate's fresh source) excludes the shared plus `[release]` lists, commit's AI diff the shared plus `[commit]` lists, and neither verb reads the other's level; commit's preflight ignores every level. Other verb keys are namespaced (`[release]`, `[commit]`); `[release].diff_keep` is read only by the exclusion check. Mint writes `.mint.toml` in exactly one place, `config.AppendReleaseGlobs`, which appends the check's answers and leaves every other byte as written. A new config key must be added to the config-metadata SoT (`config.MetadataRows()`) — the drift test in `internal/config` fails the build until the SoT and the decode-shape schema biject over `(level, key)` leaf pairs — and to the README's per-key tables (a tripwire test asserts every schema key name appears in the README). Further doc tests in `internal/config` pin README prose (the shipped default, the exclusion check, the notes floor), so a behaviour change updates that prose and its pin together; the `mint setup` guide must never mention the README. The `internal/initgen` template stays minimal (empty body + header) and carries no per-key entries, so a new key is NOT added there.

## Load-bearing invariants

- **Mutate nothing until accept** (commit): preflight probes and the would-be-committed diff are computed read-only; `git add` runs only inside the accept tail. A decline/abort leaves the index byte-for-byte untouched.
- **Surgical unwind, then PONR** (release): every pre-push mutation is tracked and unwound on failure — mint removes only what it created. The one exception is the exclusion check's answers: the unwind resets past their commit with the rest, then re-commits them on the starting commit and narrates it as kept; where git ignores an untracked `.mint.toml`, the uncommitted write is left in place. The point of no return is the single `git push --atomic origin HEAD <tag>`. After it: warn-only, never unwind.
- **Never unwind after accept** (commit): a failed `-p` push warns once (git stderr verbatim in `Warning.Output`) and keeps the commit; exit non-zero via the sentinel, no `StageFailed`.
- **Byte-identical bodies**: notes and commit messages pass through verbatim — no reformatting, no branding injected into message text (`commit_prefix` is release-bookkeeping only).
- **The notes format lives in `notes.DefaultPrompt` alone**: every format, headline and maintenance-line rule goes there. The text mint adds around the instructions on every notes call (`OutputReminder`, the chunk and piece framings, `mergeFraming`) stays format-neutral and names no headline or maintenance line, because a `[release].prompt` override replaces the prompt but that text still reaches its calls. The Change Map is a coverage check, never a ranking signal. Tests pin both sides.
- **Fail loud, never hang**: unattended (`-y` or non-TTY) paths that would need a human abort with one clear message.
- **`--notes-file` stays the user's**: a notes file inside the repo is kept out of the clean-tree check, the autostash, every commit a release makes (pre_tag artifacts, bookkeeping) or a regenerate makes (changelog), and the unwind's reset; its staged and unstaged state survives the run.
- **At most three pre-push release commits**: pre_tag artifacts, the exclusion answers (`.mint.toml` alone), and the bookkeeping commit, each counted in `MadeState`; the unwind resets to the starting HEAD and re-commits only the answers.

## Error & exit idioms

- Sentinel errors matched with `errors.Is`; wrap with `%w`; messages lowercase, no trailing punctuation.
- Failure diagnostics from subprocesses (hook output, git stderr) travel VERBATIM in `StageFailure.Output`/`Warning.Output` — never summarised or reworded.
- Exit codes: `0` success, `1` runtime failure or user abort (`cmd/mint` `exitCode()`), `2` usage errors only. `--help` prints to stdout and exits `0` (curated texts in `cmd/mint/usage.go` — a new flag needs its usage line; a test pins coverage).

## Test idioms

- Tests never spawn real `git`/`claude`/editors: script subprocesses with `runner.FakeRunner` (`Seed` keyed by command name; `SeedSequence` for same-binary call sequences, whose order is nondeterministic across concurrent chunk calls; `SeedNotFound` for a missing binary) and record output with `presentertest.RecordingPresenter`.
- External test packages (`package foo_test`), table-driven where the shape fits, `t.Parallel()` throughout, `t.TempDir()` for roots.
- Assert exact argv on git invocations and exact rendered lines on presenter output — drift in either is a contract break, not a cosmetic change.
- Behaviour-level proofs over unit minutiae: e.g. "decline runs zero `git add`/`git commit` invocations", "the push warn does not suppress `RunFinished`".

## Comments

The codebase carries heavy WHY-comments: they state contracts, invariants, and the reasoning the code can't show. Keep them TRUE TO AS-BUILT — when behaviour changes, the comment changes in the same commit. Never leave scope/phase claims that contradict the shipped code.
