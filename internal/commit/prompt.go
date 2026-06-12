// Package commit holds the commit verb's Layer-3 glue for the shared AI engine.
// Its first piece is prompt assembly: commit owns its default commit-message prompt
// and its two prompt-control knobs ([commit].context inject + [commit].prompt full
// override), then composes the finished AI input (instructions + staged diff) that
// the content-agnostic L2 transport consumes.
//
// This mirrors release's notes prompt composer (the consumed pattern) but with a
// different default — Conventional Commits 1.0.0 — and commit's two-knob context/
// override model. It owns prompt composition and the default prompt text ONLY: the
// AI call is L2's transport (internal/ai), and diff assembly + diff_exclude/
// max_diff_lines is L1 — neither is built here. The staged diff arrives as an input.
package commit

import (
	"fmt"
	"strings"

	"mint/internal/config"
)

// DefaultPrompt is the instructions mint ships when no [commit].prompt override is
// configured. It encodes commit's default format — Conventional Commits 1.0.0
// (`type: description`) — and the discipline rules the spec pins (Commit Message
// Format & Prompt): AI infers the type, scope omitted by default, an imperative
// concise subject with an optional wrapped body for the why, and a plain message
// with no mint branding / commit_prefix and no preamble or meta-commentary.
//
// It is the "instructions" half of the composed AI input; ComposePrompt appends the
// staged diff after it. Wording is deliberately ship-and-refine (tuned over real
// commits); every rule below is required and individually assertable in the tests.
const DefaultPrompt = `You are writing the git commit message for a set of staged
changes. You are given the staged diff below. Read the WHOLE diff and produce a
single commit message in the Conventional Commits 1.0.0 format described below.

Subject line:

- Write a "type: description" subject — a Conventional Commits type followed by a
  colon, a space, and a short description. Example: "fix: handle empty staged diff".
- Read the diff and infer the type from it (feat, fix, chore, docs, refactor, test,
  build, ci, perf, style, …) — choose the one that best fits what the diff does.
- Omit the scope. Do NOT guess a "(scope)" — scope conventions are project-specific
  and guessing them is worse than leaving them out. Emit "type: description", never
  "type(scope): description".
- Keep the description imperative ("add", "fix", "remove" — not "added"/"fixes")
  and concise: one short line summarising the change.

Body (optional):

- If the why is not obvious from the subject, add a blank line after the subject,
  then a wrapped body explaining the motivation — WHY the change was made, not a
  restatement of WHAT changed. Wrap the body at a sensible width. Omit the body
  entirely when the subject already says enough.

Output discipline:

- Emit a plain conventional-commit message and nothing else. No mint branding, no
  prefix or emoji, no decoration around the message.
- Return the commit message directly. Do NOT wrap it in any machine-parseable
  labels, headings, code fences, or markers.
- Strict: no preamble and no meta-commentary. Do not explain what you are doing, do
  not restate these instructions, do not add a sign-off. Output only the message.`

// ComposePrompt assembles the final AI input from its two parts, in EXACTLY this
// order and nothing else:
//
//	{instructions} + {staged diff}
//
// instructions is the resolved prompt (the default prompt, the default plus an
// injected [commit].context, or a full [commit].prompt override — see
// ResolveInstructions). diff is the staged-diff content supplied by L1 (this unit
// does not collect it). The function is PURE: it only orders and joins, performing
// no IO and no transport — it produces the AI INPUT, never parses the AI output.
//
// The diff always sits in the trailing position, including under a full prompt
// override (the override replaces the instructions segment only; the diff is still
// appended here, never dropped or reordered). The two parts are separated by a blank
// line so the AI sees two clearly delimited blocks; no labels or machine-parseable
// wrappers are added (the AI returns the commit message directly).
func ComposePrompt(instructions, diff string) string {
	return strings.Join([]string{instructions, diff}, "\n\n")
}

// contextHeader labels the injected [commit].context block appended to the default
// prompt so the AI reads it as supplementary project guidance, not as part of the
// diff.
const contextHeader = "Additional project context (apply the rules above with this in mind):"

// oneTimeContextHeader labels the one-time regenerate context block injected by the
// gate's `r` (regenerate-with-context) action. It is distinct from contextHeader so
// the AI reads it as the user's one-shot steer for THIS regeneration only — it is
// never persisted to [commit].context and is layered ON TOP of any persisted context.
const oneTimeContextHeader = "Additional one-time context for this regeneration (apply the rules above with this in mind):"

// ResolveInstructions produces the "instructions" half of the AI input from
// commit's two prompt-control knobs, reading the override file relative to root via
// config.ResolveCommitPrompt. This is the IO side of prompt assembly (it may read a
// file); ComposePrompt is the pure side.
//
// Resolution, in precedence order:
//
//   - [commit].prompt set → it is a FILE PATH whose contents fully OVERRIDE the
//     default prompt (the default is replaced entirely). Context is ignored in this
//     case — context injects into the default prompt, and a full override has no
//     default to inject into. An unreadable override file is an error, NOT a silent
//     fall-back to the default: a configured prompt is an explicit operator choice.
//   - else → the DEFAULT prompt, with [commit].context (if set) INJECTED (appended,
//     not replacing) so every default rule survives alongside the project guidance.
//   - neither set → the default prompt verbatim.
func ResolveInstructions(cfg config.Config, root string) (string, error) {
	override, err := config.ResolveCommitPrompt(cfg, root)
	if err != nil {
		return "", fmt.Errorf("resolving commit prompt: %w", err)
	}
	if override != "" {
		return override, nil
	}

	if cfg.Commit.Context == "" {
		return DefaultPrompt, nil
	}
	return injectContext(DefaultPrompt, contextHeader, cfg.Commit.Context), nil
}

// injectContext appends context to prompt under a labelled header, injecting (never
// replacing) so every prior rule survives alongside the added guidance. The header is
// a parameter so the SAME idiom serves both the persisted [commit].context block
// (contextHeader) and the gate's one-time regenerate block (oneTimeContextHeader).
func injectContext(prompt, header, context string) string {
	return prompt + "\n\n" + header + "\n\n" + context
}

// injectOneTimeContext layers the gate's `r` one-time context onto resolved
// instructions, ON TOP of any already-injected [commit].context, under the
// oneTimeContextHeader. An EMPTY context is a no-op — a plain re-roll injects no
// block, so the instructions are byte-identical to a normal generate. The one-time
// context is a local string only; it is never written back to cfg/[commit].context.
func injectOneTimeContext(instructions, oneTimeContext string) string {
	if oneTimeContext == "" {
		return instructions
	}
	return injectContext(instructions, oneTimeContextHeader, oneTimeContext)
}
