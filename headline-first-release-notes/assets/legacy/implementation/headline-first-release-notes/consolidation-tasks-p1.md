# Consolidation Tasks: Headline First Release Notes (Phase 1)

## Task 1: Corrections
placement: phase 1
severity: corrections

**Problem**: The default prompt asks for the maintenance line "in the shape of" the empty-diff stub by restating the stub's wording as its own literal (`internal/notes/prompt.go:183`), beside `stubBody` (`internal/notes/degenerate.go:13`), whose doc (`degenerate.go:5-8`) says the wording is "tunable in one place" and which `StubBody()` (`degenerate.go:45-46`) hands to the empty-diff path. Retuning `stubBody` would leave the AI path asking for the old line, so a release with nothing users meet and an empty-diff release would ship differently worded maintenance entries on the tag, the CHANGELOG and the provider release. Each side is pinned by its own literal (`degenerate_test.go:95`, `prompt_test.go:644`), so nothing catches the drift. The specification ties the two together: §5.1 asks for the maintenance line "in the shape of the empty-diff stub".
**Solution**: One edit: `internal/notes/prompt.go:183` — build the bullet's quote from the constant, `` in the shape of "` + stubBody + `". That line ``, breaking the raw string around it the way `prompt.go:60` already splices in the `ai_command` example. Both are untyped string constants, so `DefaultPrompt` stays a const and its bytes stay identical; the existing prompt and stub tests stay green and the "tunable in one place" claim on `stubBody` holds again.

**Outcome**: The maintenance wording has one production source, `stubBody`. `DefaultPrompt` takes its quote from that constant, so a retune of `stubBody` changes the empty-diff stub and the line the AI is asked for together. Today's `DefaultPrompt` bytes, every notes call built from them, and every test are unchanged.

**Acceptance Criteria**:
- [ ] `DefaultPrompt` is byte-identical to the prompt before the edit: the maintenance bullet still reads `in the shape of "Maintenance release — no notable source changes". That line` and then `has no headline and no sections`, with the same wrapping and indentation. `TestDefaultPrompt_AsksForMaintenanceLineWhenNothingUsersMeet` and `TestDefaultPrompt_GivesMaintenanceLineNoHeadlineAndNoSections` pass with no edits.
- [ ] `DefaultPrompt` is still declared `const`, and its maintenance quote is the `stubBody` identifier, not a literal. Run from the repo root, `rg -n 'Maintenance release — no notable source changes' --type go -g '!*_test.go'` returns two lines today (`internal/notes/degenerate.go:13`, `internal/notes/prompt.go:183`) and returns only `internal/notes/degenerate.go:13` after the edit.
- [ ] No test file changes. The pins at `internal/notes/degenerate_test.go:95` and `internal/notes/prompt_test.go:644`, and the engine tests that pin the same wording, keep their own literals. `go build ./...`, `gofmt -l .` (no output), `go vet ./...`, `go test -race ./...` and `golangci-lint run` (0 issues) all pass.

**Do**:
- `internal/notes/prompt.go:183`: close the raw string after `in the shape of "`, concatenate `stubBody`, and reopen it with `". That line`. Use the same splice `prompt.go:60` uses for the `ai_command` example. No other line of the prompt changes.
- The measured set is the two non-test literals the grep above returns, and the edit leaves one. The nine `_test.go` hits of the same grep without the `-g` filter are independent pins of the shipped bytes. They stay as written, and so does `internal/notes/degenerate.go`.
