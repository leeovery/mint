// Package aitransport is the ONE place mint maps a resolved config plus a verb to a
// production ai.Transport. It exists because the three transport-wiring sites — the
// release notes engine (internal/engine/release.go), the regenerate fresh-source path
// (internal/engine/regenerate_fresh.go), and the commit verb (internal/commit/run.go) —
// each construct the SAME `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb),
// Timeout: cfg.TimeoutFor(verb)})` expression and differ only in the verb constant they
// pass. Holding that expression here keeps the "thread BOTH the command and the timeout
// from the per-verb accessor, never zero-by-omission" contract in a single editable place.
//
// It lives in its own package because engine and commit are independent sibling packages
// (neither imports the other), so a helper they BOTH consume cannot live in either. It
// also cannot live in ai (the transport must stay content-agnostic — it must not learn
// about config, per CLAUDE.md seam #5) nor in config (config must not import ai). This
// package is the only seam-clean home: it imports ai + config + runner, and no cycle
// exists because config does not import ai and ai does not import config.
package aitransport

import (
	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/runner"
)

// New builds the production ai.Transport for verb over r, sourcing BOTH the concrete
// command and the per-attempt deadline from the verb's per-key config accessors:
// cfg.AICommandFor(verb) and cfg.TimeoutFor(verb) each walk the chain
// `[verb] → shared → floor`, so a per-verb override drives the call and a zero-config run
// resolves to the shipped floor (claude -p --model sonnet, 60s). Config — not the transport
// — owns the default and the blank-skip / no-deadline semantics; the transport runs what it
// is handed (whitespace-splitting the command into name + args).
//
// The timeout is sourced from the accessor (a *time.Duration assigned DIRECTLY to
// ai.Config.Timeout, never zero-by-omission), so "no deadline" stays reachable ONLY via an
// operator's explicit `0` — a forgotten field cannot reach the transport (it would surface
// as a nil ai.Config.Timeout, which NewTransport panics on).
//
// The verb is the closed config.Verb enum (no `regenerate` value), so each call site can
// only pass one of the two real verb tables: release and the regenerate fresh path pass
// VerbRelease (regenerate rides on [release]), commit passes VerbCommit.
func New(r runner.CommandRunner, cfg config.Config, verb config.Verb) *ai.Transport {
	return ai.NewTransport(r, ai.Config{
		AICommand: cfg.AICommandFor(verb),
		Timeout:   cfg.TimeoutFor(verb),
	})
}
