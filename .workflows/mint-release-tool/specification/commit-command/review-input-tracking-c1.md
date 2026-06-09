---
status: in-progress
created: 2026-06-09
cycle: 1
phase: Input Review
topic: Commit Command
---

# Review Tracking: Commit Command - Input Review

## Findings

### 1. Gate-rendering reconciliation hand-off (cli-presentation `[a]/[q]`→`Continue?`) applies to commit's gate

**Source**: discussion/commit-command.md, "Spec hand-offs" #3 (lines 650-651): *"Gate semantics already owed by release (cli-presentation's `[a]/[q]`→`Continue?` reconciliation) apply to commit's gate too — commit consumes the same rendering."*
**Category**: Enhancement to existing topic
**Affects**: Dependencies (Notes), and/or Interactive Review Gate

**Details**:
The discussion records three spec hand-offs / reconciliations owed by the in-progress release spec. The specification captures hand-off #1 (config restructure → verb-namespaced shape — covered in Config Schema and Dependencies) and hand-off #2 (shared AI engine three-layer split — covered in AI Engine and Dependencies). It does **not** capture hand-off #3: that the cli-presentation gate-rendering reconciliation (`[a]/[q]`→`Continue?`) which release already owes also flows through to commit's gate, because commit consumes the exact same `Continue?` rendering.

This matters because commit's review gate is a hard dependency on cli-presentation's *reconciled* gate rendering. If that reconciliation is tracked only as a release-spec obligation, the cross-dependency to commit (commit inherits whatever the reconciled gate becomes) is left implicit. Capturing it keeps the dependency chain explicit and mirrors how the spec already surfaces the config and engine reconciliations.

**Current**:
(Dependencies → CLI Presentation row, line 247)
> | **CLI Presentation** (`cli-presentation` spec) | Commit renders *all* output and its review gate through the `Presenter` seam — pretty/plain by `isatty`/`--plain`, `-y` auto-accept, the `Continue?` gate rendering, and the shared non-TTY forbidden-combo rule. None of commit's interactive flow can be built without this seam. | The entire commit presentation surface: gate rendering, pretty/plain modes, `--plain`/`-y` handling, and the fail-loud forbidden-combo behaviour. |

**Proposed Addition**:
(leave blank until discussed — likely a Notes bullet under Dependencies stating that the cli-presentation gate reconciliation owed by the release spec, `[a]/[q]`→`Continue?`, applies to commit's gate too since commit consumes the same rendering)

**Resolution**: Pending
**Notes**:

---
