TASK: mint-release-tool-2-11 — Single-body distribution to all sinks

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, matches ACs and spec exactly. engine/release.go threads resolved body whole into all three sinks: record.WriteChangelog(..., body, cfg.Release.Changelog) (skipped when changelog=false), Releaser.TagAndPush(..., body) (unconditional), publisher.CreateRelease(..., body) (gated on Publish + non-nil publisher). Same body var reaches every sink. Sink-side: tag message subject+"\n\n"+body, changelog verbatim %s, provider via --notes-file - stdin — no parsing/splitting. Toggles *bool absent→default-true via boolOrDefault. changelog=false avoids empty bookkeeping commit; tag still carries full body.

TESTS:
- Status: Adequate. SingleBodyToAllSinks (identical bytes + cross-check equal), ChangelogDisabled (no CHANGELOG, no bookkeeping commit, tag carries body), PublishDisabled (no gh, tag carries body), ChangelogDefaultTrue, EmptyNotesBody fallback, edit gate body reaches all three sinks verbatim. Config toggle defaults pinned. Sink-level body-whole in unit tests.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, owned symbols prevent drift). SOLID good — orchestrator calls existing sink units unchanged. Release long but linear/commented. Modern idioms, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
