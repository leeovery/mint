TASK: Bring the README's exclusion-check text up to what 7-8 and 7-13 built (token-aware-diff-sizing-7-15, tick-185c1b)

ACCEPTANCE CRITERIA:
- In the README's `release` section, the paragraph on the check's call says that once the list would take more than a tenth of `max_prompt_bytes`, it condenses by folder, with a folder's paths summed up as a count and a total diff size (§6.2, §6.3)
- The pause paragraph says that a pause shows, after the glob and the AI's reason, how many changed files the notes still carry that the glob matches, the total size of their diffs, and any of them on `[release].diff_keep` or kept earlier in the run, and that an Exclude takes those kept files out too (§6.5, §5.2)
- The no-pause sentence also covers a suggestion whose matches earlier answers in the run already decide, alongside a suggestion that matches no changed path, or only paths a `diff_exclude` level or `[release].diff_keep` already covers (§6.5)
- The failure paragraph adds a list that will not fit in `max_prompt_bytes` even condensed to the check's failure causes. It still says mint warns `couldn't check for files that don't belong in the notes`, with the AI's output beneath it where there is any, and carries on with no suggestions (§6.8)
- `TestREADME_ReleaseSectionDescribesTheExclusionPauses` pins the new pause sentence through its count, size and keep-list clause; the new pin reaches past "the AI's reason" and fails against the old README (§18.2)
- `TestREADME_ReleaseSectionDescribesTheFailedCheck` pins the new failure cause and fails against the old README; every other pin in `readme_exclusion_check_test.go` passes unchanged (§18.2)
- Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass

STATUS: complete

SPEC CONTEXT: §6.2/§6.3 — the check's path list condenses by folder once it would take more than a tenth of the cap (example `vendor/: 4,312 files, 38 MB`); a list that cannot get under the cap even condensed is a failed check. §6.5 — a pause shows the glob and reason, the count and total diff size of the changed paths the notes diff still carries at that pause (keep-list paths counted), and any of them on the keep list including earlier Keep answers; a suggestion whose remaining matches earlier answers already cover raises no pause. §5.2 — the keep list never overrides an exclusion. §6.8 — five failure causes, one warning line with the AI's output beneath where there is any, release carries on. §18.2 — the check and its pauses documented as built.

IMPLEMENTATION:
- Status: Implemented
- Location: README.md:115 (condensing sentence), README.md:117 (pause content and extended no-pause sentence), README.md:119 (fifth failure cause, "then no call is made"); internal/config/readme_exclusion_check_test.go:29-30 (pause pin replaced, no-pause pin added), internal/config/readme_exclusion_check_test.go:77 (failure-cause pin added)
- Notes: Every claim checked against the code as built.
  - Condensing: internal/notes/condense.go:10-11 (share = maxPromptBytes / 10); internal/notes/exclusioncheck.go:306-321 condenses only when the rendered list exceeds the share; internal/notes/exclusioncheck.go:330-332 renders `{folder}/: {count} files, {size}` with groupThousands and bytesize.Format, so the README's `vendor/: 4,312 files, 38 MB` is a shape the code produces.
  - Pause: internal/engine/release.go:1210-1234 renders `{n} files, {size}` then `; on the keep list: …`; internal/notes/exclusioncheck.go:228-241 counts paths still in the post-exclusion diff not covered by an earlier Exclude, keep-list paths included, and Kept covers both the configured list and this run's Keep answers (exclusioncheck.go:207-214). "Which an Exclude takes out too" holds per §5.2.
  - No-pause: usableSuggestions (exclusioncheck.go:249-263) drops suggestions matching no uncovered path; pause() skips via leavesUndecided (exclusioncheck.go:179-181, 222-226) when earlier answers decide every match. The extended sentence matches both.
  - Over-cap failure: exclusioncheck.go:129-133 calls CheckFailed("") without Checking() or a transport call, so "then no call is made" and nothing beneath the warning are both accurate.
  - The commit (9e4ac08) touched only README.md and internal/config/readme_exclusion_check_test.go; no behaviour change.

TESTS:
- Status: Adequate
- Coverage: The pause pin (readme_exclusion_check_test.go:29) runs through the count, size and keep-list clause and past "the AI's reason", which the old README ended on with a full stop, so it fails against the old text. The added no-pause pin (:30) covers the "earlier answers in the run already decide" extension. The failure-cause pin (:77) is absent from the old README. The other pins are unchanged, and each is a substring of the current whitespace-flattened `### `release`` section (README.md:83-140), checked by reading README.md:115-121 against every pin.
- Notes: The condensing sentence at README.md:115 is not pinned; no criterion asks for that pin, so this is not a finding.

CODE QUALITY:
- Project conventions: Followed (package config_test, t.Parallel(), flattened-section substring pins consistent with sibling README tripwires)
- SOLID principles: Good (N/A for a doc change)
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass" — the Go-file half is settled by reading the commit; whether `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` all pass needs those five commands run on the current tree.
