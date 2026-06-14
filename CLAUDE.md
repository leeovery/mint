# CLAUDE.md

Mint is a Go CLI that cuts releases (`mint release`), regenerates release notes (`mint release regenerate`), and mints AI-generated commits (`mint commit`). See README.md for the user-facing surface. This file is the working contract for changing the code.

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

- `cmd/mint` — thin entry: flag parsing (stdlib `flag`, one file per verb), seam construction, dispatch, exit-code mapping. No business logic.
- `internal/engine` — the release/regenerate/init orchestrators (the release spine, surgical unwind, hooks, dry-run cache).
- `internal/commit` — the `mint commit` orchestrator (gate loop, editor fallback, deferred staging, push).
- `internal/presenter` — the ONLY output surface: pretty + plain renderers, gates, prompts. `presentertest.RecordingPresenter` is the test double.
- `internal/runner` — the ONLY subprocess surface: `CommandRunner` (`Run`/`RunWith`/`RunInDir`/`RunInteractive`). `FakeRunner` is the test double.
- `internal/git` — `Mutator`, the lock-resilient git mutation sink. `internal/gitrepo` — read-side repo resolution.
- `internal/ai` — content-agnostic AI transport (finished prompt in, body out). `internal/aitransport` — the ONE place mapping a resolved config + verb to a production `ai.Transport` (the three wiring sites — release, regenerate, commit — consume it). `internal/notes` — release-specific context assembly (L1); exposes the reusable `CheckDiffSize`/`ErrDiffTooLarge`.
- `internal/config` — the canonical `.mint.toml` schema, strict decoding. `internal/initgen` — the pure `mint init` template generator.
- Supporting: `internal/version` (SemVer), `internal/publish` (provider drivers), `internal/record` (bookkeeping commit), `internal/hooks`, `internal/notescache`, `internal/preflight`, `internal/release`, `internal/buildinfo`, `internal/fsutil`.

## Non-negotiable seams

1. **Every external command goes through `runner.CommandRunner`.** Never `os/exec` directly. `RunInteractive` is ONLY for handing the terminal to a child (the user's `$EDITOR`). Read-side git that uses cwd-relative pathspecs (`-- .`, `ls-files`) must run at the repo root via `RunInDir(root, …)` — a plain `Run` from a subdirectory silently scopes to the subtree.
2. **Every git MUTATION goes through `git.Mutator`** (`Mutate(ctx, stdin []byte, name, args...)`) so it is lock-resilient. Reads stay on the plain runner. Bodies are passed as `[]byte`, never a shared `io.Reader` (a retry must re-pipe the full payload).
3. **Every byte of user output goes through `presenter.Presenter`.** No `fmt.Print`/`os.Stdout`/`os.Stderr` in business logic (the cmd layer's usage/error lines are the only exception). Events are payload-driven — the presenter never re-derives or invents narration. `NewForStartup(plainFlag, yes, stdout, stderr, stdin)` is the single production construction site; TTY detection never happens downstream.
4. **Gates:** business logic ALWAYS calls `Prompt(gate)` — `-y` auto-accept (and its echo) happens INSIDE the presenter, never as an engine-side branch. `ErrNotInteractive` is already rendered by the presenter (wrap it, add no narration); `ErrInputClosed` is NOT rendered (the caller surfaces it). Free-text reads go through `AskLine` — the engine never reads stdin.
5. **AI goes through `internal/ai`** behind a consumer-defined one-method interface at the point of use (see `notes/generate.go` and `commit/generate.go`). Production transports are built in exactly one place — `aitransport.New(runner, cfg, verb)` — which sources BOTH the command and the per-attempt timeout from the per-verb config accessors (never zero-by-omission); the transport stays content-agnostic and never imports `config`. The transport owns validation and the single bad-content retry — consumers never re-retry. Sentinels: `ErrGenerationFailed` / `ErrTimeout` / `ErrCommandMissing`; `context.Canceled` propagates unchanged (a cancel is not an AI failure — never route it to a fallback).
6. **Config is strict.** All keys live in `internal/config`'s canonical schema; `DisallowUnknownFields` rejects unknowns; new keys need a `typeErrorMessages` entry for a friendly type error. `ai_command` and `timeout` live at BOTH levels — a top-level shared baseline plus an optional per-verb override — resolved per-key `[verb] → shared → shipped default` by the typed `AICommandFor`/`TimeoutFor` accessors (keyed on the closed `Verb` enum, which has no `regenerate` value — regenerate rides on `[release]`); `max_diff_lines`/`diff_exclude` stay shared-only at the top level. Other verb keys are namespaced (`[release]`, `[commit]`). New config keys must also be added to the commented template in `internal/initgen`.

## Load-bearing invariants

- **Mutate nothing until accept** (commit): preflight probes and the would-be-committed diff are computed read-only; `git add` runs only inside the accept tail. A decline/abort leaves the index byte-for-byte untouched.
- **Surgical unwind, then PONR** (release): every pre-push mutation is tracked and unwound on failure — mint removes only what it created. The point of no return is the single `git push --atomic origin HEAD <tag>`. After it: warn-only, never unwind.
- **Never unwind after accept** (commit): a failed `-p` push warns once (git stderr verbatim in `Warning.Output`) and keeps the commit; exit non-zero via the sentinel, no `StageFailed`.
- **Byte-identical bodies**: notes and commit messages pass through verbatim — no reformatting, no branding injected into message text (`commit_prefix` is release-bookkeeping only).
- **Fail loud, never hang**: unattended (`-y` or non-TTY) paths that would need a human abort with one clear message.

## Error & exit idioms

- Sentinel errors matched with `errors.Is`; wrap with `%w`; messages lowercase, no trailing punctuation.
- Failure diagnostics from subprocesses (hook output, git stderr) travel VERBATIM in `StageFailure.Output`/`Warning.Output` — never summarised or reworded.
- Exit codes: `0` success, `1` runtime failure or user abort (`cmd/mint` `exitCode()`), `2` usage errors only. `--help` prints to stdout and exits `0` (curated texts in `cmd/mint/usage.go` — a new flag needs its usage line; a test pins coverage).

## Test idioms

- Tests never spawn real `git`/`claude`/editors: script subprocesses with `runner.FakeRunner` (`Seed` keyed by command name; `SeedSequence` for same-binary call sequences) and record output with `presentertest.RecordingPresenter`.
- External test packages (`package foo_test`), table-driven where the shape fits, `t.Parallel()` throughout, `t.TempDir()` for roots.
- Assert exact argv on git invocations and exact rendered lines on presenter output — drift in either is a contract break, not a cosmetic change.
- Behaviour-level proofs over unit minutiae: e.g. "decline runs zero `git add`/`git commit` invocations", "the push warn does not suppress `RunFinished`".

## Comments

The codebase carries heavy WHY-comments: they state contracts, invariants, and the reasoning the code can't show. Keep them TRUE TO AS-BUILT — when behaviour changes, the comment changes in the same commit. Never leave scope/phase claims that contradict the shipped code.
