TASK: cli-presentation-4-4 — Verb-shaped, success-only end-of-run line (release footer with url; regenerate without url; suppressed on failure)

ACCEPTANCE CRITERIA:
- The end-of-run payload carries a verb/shape discriminator and an optional URL; RecordingPresenter captures the shape.
- A successful release renders `🌿 released {project} v{X} · {url}` (pretty) / `done: {project} v{X} {url}` (plain) — with the URL.
- The release footer renders the engine-supplied brand leaf (RunResult.Leaf, default 🌿), not a hardcoded literal.
- A successful regenerate renders the closing summary without the URL (no dangling ·) in both modes.
- init and version render no end-of-run footer.
- When the success-suppression flag is set (failure or abort), RunFinished renders nothing for every verb shape — suppression precedes shaping.
- The presenter never re-derives the verb — the shape comes from the payload; Warn alone does not suppress the footer.

STATUS: Complete

SPEC CONTEXT: "Cross-Verb Rendering" (spec:284-288) — end-of-run is success-shaped AND verb-shaped; release-success form has URL; regenerate URL-less summary; init terminal (no footer); failure suppresses success line. Pretty Layer (spec:124) bottom brand line ties leaf to commit_prefix; Plain Layer (spec:228) done: form. Extends 2-8 suppression flag to verb-shaping (suppression precedes shaping).

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:441-509 (RunVerb enum VerbRelease iota-0 + RunResult.Verb/Summary; appended verbs keep release as zero-value); plain.go:428-440 (RunFinished suppression-first + exhaustive switch), :445-451 (renderReleaseFooter); pretty.go:881-893, :899-906 (renderReleaseFooter), :339 (leafOrDefault); presentertest/recording.go:220-221.
- Notes: Suppression checked BEFORE verb switch (plain.go:429, pretty.go:882); terminalFailure (set by StageFailed/Unwound, not Warn) wins over every shape. Release footer renders r.Leaf via leafOrDefault, never hardcoded; VerbRegenerate also uses leafOrDefault. URL only on release; regenerate omits it (no dangling sep); init/version render nothing; empty-URL release collapses cleanly. Footer narration → out only (no errf). Regenerate set-summary owned by 4-2; this task renders engine Summary verbatim. Additive discriminator — prior Verb-less literals render release form unchanged.

TESTS:
- Status: Adequate
- Coverage (runfinished_dispatch_test.go + reinforcing plain/pretty): release footer with url both modes (:23-51); leaf from payload + empty→🌿 (:58-81); regenerate close no url, no dangling sep (:86-110); init no footer (:116-140); version no footer (:146-170); failure suppresses despite URL (:176-202); abort (Unwound no prior StageFailed) suppresses (:208-237); suppression-precedes-shaping table over all 4 verbs both modes (:245-284); warn-only still emits footer (:290-319); supporting --all set-summary + empty-URL collapse + default-verb regression in plain/pretty tests.
- Notes: Every AC + edge mapped. Pretty under termenv.Ascii for deterministic bytes. Dispatch suite defers content to plain/pretty tests (intentional separation). Exact-match footers.

CODE QUALITY:
- Project conventions: Followed — writef→out, shared leafOrDefault, exhaustive switch with explicit no-footer arm + comment.
- SOLID principles: Good — single responsibility; presenter renders engine-supplied shape, never re-derives verb.
- Complexity: Low — guard + four-arm switch; URL-empty branch extracted.
- Modern idioms: Yes — iota enum, typed discriminator, additive fields.
- Readability: Good — explicit no-footer arm.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
