# Plain-mode prompts ignore Ctrl-C until Enter is pressed

When mint runs in plain mode with an interactive stdin, pressing Ctrl-C at a prompt does not stop the run. That covers `--plain` at a terminal, or a release piped through something like `mint release | tee log`. Mint sits blocked at the prompt as if hung, and nothing happens until the operator presses Enter or stdin closes. Every plain-mode prompt behaves this way: the release review gate, the version confirm, the notes floor's `c` / `e` / `a` choice, the exclusion-check pauses and free-text reads. Pretty mode is not affected, because its raw-mode keypress read sees Ctrl-C itself.

The first Ctrl-C is caught by mint's signal handling (`cmd/mint/main.go:74`), and all it does is cancel the run's context. The plain prompt is blocked reading a line from stdin (`readChoice` in `internal/presenter/prompt.go:87`) and is not tied to that context, so the read carries on waiting. The exclusion pause's adapter also discards the context it is handed (`internal/engine/release.go:1190`).

What happens after that depends on the prompt:

- **Exclusion-check pauses:** when the operator finally presses Enter, the check notices the cancel straight after the pause returns (`internal/notes/exclusioncheck.go:185`), and the run aborts with nothing recorded.
- **Review gate and notes floor:** the Enter is taken as an answer before the cancel is noticed. At the review gate that is the default, accept.
- **A second Ctrl-C:** it kills the process outright, because mint restores default SIGINT handling after the first one (`cmd/mint/main.go:84-88`). That kill skips the surgical unwind, so a commit mint made of the `pre_tag` hook's artifacts earlier in the run is left on the branch.

The token-aware-diff-sizing specification (§6.6) says a Ctrl-C during the exclusion pauses cancels the run. In plain mode that holds only once Enter is pressed. More broadly, the operator gets no way to back out promptly at any plain-mode prompt. The obvious escalation, pressing Ctrl-C again, either commits to an answer they did not mean or leaves a stray commit behind.

The defect predates the token-aware-diff-sizing work; the new exclusion pauses inherited it. It was raised in that work's review, cycle 2 (source finding `1-purpose-and-scope-2-the-lean-default-ai-command-3-quote-aware-ai-command-parsing-4-the-per-call-size-cap-5-per-verb-exclusion-lists-6-the-exclusion-check-7-routing-a-release-s-notes-8-chunked-generation-1`, banked as out-of-scope A2). It was filed here rather than fixed in that review.

Guards to keep in mind around the presenter's input handling:

- The presenter's sources may not import `syscall` or `os/exec`: `TestPromptPathImportsNoSubprocessDependency`.
- The `-y` and non-TTY paths must never read stdin: the `failingReader` no-stdin-read tests.
- `Prompt` and `AskLine` share one buffered reader that consumes lines in order: `TestPlainPromptThenAskLineConsumeConsecutiveLines`.

Files involved: `internal/presenter/prompt.go`, `internal/presenter/plain.go`, `internal/engine/release.go`, `cmd/mint/main.go`, `internal/notes/exclusioncheck.go`.
