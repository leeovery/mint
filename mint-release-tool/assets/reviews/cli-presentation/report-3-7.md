TASK: cli-presentation-3-7 — Regenerate source/target prompts reuse the line-read model, skip under -y, obey forbidden-combination (tick-a540d2)

ACCEPTANCE CRITERIA:
- Source and target prompts render through the shared Prompt(gate) method (no second input loop).
- Interactive input uses the shared line-read model: case-insensitive, empty Enter -> default, unrecognised re-prompts.
- Plain renders the prompts as terse key:value prompt lines.
- Under -y, prompts skipped using provided flag/default; chosen value echoed ('source: {chosen} (-y)' / 'target: {chosen} (-y)' plain; concise pretty accept line).
- Non-TTY stdin without -y fails loud (through presenter and to stderr), reusing the forbidden-combination path.
- When skipped under -y, the flag/default value is used (engine-provided default), not an interactive read.

STATUS: Complete

SPEC CONTEXT: "Gating & -y Orthogonality" — gate inventory lists regenerate source+target prompts (spec:91); the source/target paragraph (spec:116) requires same line-read model as Continue?, plain terse key:value lines, skipped under -y with echo, forbidden-combination. Intent: prove the 3-1..3-6 machinery generalises via reuse.

IMPLEMENTATION:
- Status: Implemented
- Location: gate.go:193-201 (SourceGate), :219-227 (TargetGate) — build Gate from engine options+default; Subject "source"/"target", AcceptEcho=string(def) so -y echo carries chosen value; plain.go:345-358 + pretty.go:745-757 — SAME shared Prompt handles every gate (-y skip+echo, forbidden fail, interactive readChoice all reused unchanged); prompt.go:47-95 (shared core), :102-109 (plainKeyHint).
- Notes: No second loop, no parallel skip/fail logic. AcceptEcho generalisation (echo word in payload) is the only model change, preserves notes/reuse echo (regression-guarded). Engine-side wiring of real sources/targets out of scope; constructors take options/def as args and invent nothing.

TESTS:
- Status: Adequate
- Coverage (gate_sourcetarget_test.go): re-prompt then accept (:82, "Source?" rendered exactly twice); plain terse key:value lines (:101, no pretty menu artefacts); -y value echo plain+pretty (:132, failingReader); target plain echo (:164); forbidden-combo fail-loud (:196, errors.Is, both streams, reader untripped); flag/default used when skipped (:224, different default "gitlab"); case-insensitive + empty-Enter default (:244); byte-purity (:183); constructor payload (:39,:60); notes/reuse echo regression guard (:268).
- Notes: Every AC + listed bullet mapped. failingReader/tripped checks load-bearing. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — documented constructors, package test style, reused helpers.
- SOLID principles: Good — single Prompt renders every variant (open/closed); Gate pure data model; source/target genuinely same code path.
- DRY: Good — no duplicated input/skip/fail logic; echo word lifted onto AcceptEcho.
- Complexity: Low.
- Modern idioms: Yes — typed Choice, shared free-function core.
- Readability: Good — ASCII precondition on option values documented.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/gate.go:193 — plain interactive render is "Source? [github/gitlab]" ({Question} [hint]), reusing the 3-3 form. The spec phrase "terse prompt lines in the same key: value vocabulary" could also be read as a literal "source: [github/gitlab]" prompt. Task description resolves toward 3-3 consistency and the -y echo IS literally "source: …", so current form is defensible. Flag only for spec-author confirmation; no change needed unless the literal form was intended.
