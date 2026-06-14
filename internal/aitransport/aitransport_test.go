package aitransport_test

// Proofs for the shared transport-construction helper (ai-model-selection-4-1): the one
// place the production `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb),
// Timeout: cfg.TimeoutFor(verb)})` expression lives, consolidating the three formerly
// duplicated wiring sites. New maps a (runner, config, verb) triple to a constructed
// transport, sourcing BOTH the command and the per-attempt deadline from the verb's
// accessors. The command is observable as the invoked argv; the deadline is observable as
// the presence/absence of a context deadline on the call the transport hands the runner —
// config's explicit 0 ("no deadline") must reach the transport as a deadline-free context,
// while a positive value must reach it as a context carrying a deadline. The FakeRunner
// discards the context, so these proofs use runner.DeadlineRecordingRunner.

import (
	"testing"
	"time"

	"mint/internal/aitransport"
	"mint/internal/config"
	"mint/internal/runner"
)

// TestNew_SourcesCommandAndDeadlineFromVerbAccessor proves the helper threads BOTH the
// per-verb command (cfg.AICommandFor(verb), observed as the invoked argv) and the per-verb
// deadline (cfg.TimeoutFor(verb), observed as the context deadline). It is table-driven
// across the two verbs so the same construction is proven to resolve through each verb's
// own table: VerbRelease reads [release], VerbCommit reads [commit].
func TestNew_SourcesCommandAndDeadlineFromVerbAccessor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		verb     config.Verb
		cfg      config.Config
		wantName string
		wantArgs []string
		wantDead bool
	}{
		{
			name: "release verb resolves command and positive deadline through [release]",
			verb: config.VerbRelease,
			cfg: config.Config{
				AICommand: "shared-bot",
				Release:   config.Release{AICommand: strPtr("releasebot run --json")},
				Commit:    config.Commit{AICommand: strPtr("wrongbot")},
				Timeout:   runner.DurationPtr(90 * time.Second),
			},
			wantName: "releasebot",
			wantArgs: []string{"run", "--json"},
			wantDead: true,
		},
		{
			name: "commit verb resolves command and positive deadline through [commit]",
			verb: config.VerbCommit,
			cfg: config.Config{
				AICommand: "shared-bot",
				Release:   config.Release{AICommand: strPtr("wrongbot")},
				Commit:    config.Commit{AICommand: strPtr("commitbot run --json")},
				Timeout:   runner.DurationPtr(90 * time.Second),
			},
			wantName: "commitbot",
			wantArgs: []string{"run", "--json"},
			wantDead: true,
		},
		{
			name: "release verb honours an explicit zero timeout as no deadline",
			verb: config.VerbRelease,
			cfg: config.Config{
				AICommand: "claude",
				Release:   config.Release{Timeout: runner.DurationPtr(0)},
				Timeout:   runner.DurationPtr(60 * time.Second),
			},
			wantName: "claude",
			wantArgs: []string{},
			wantDead: false,
		},
		{
			name: "commit verb honours an explicit zero timeout as no deadline",
			verb: config.VerbCommit,
			cfg: config.Config{
				AICommand: "claude",
				Commit:    config.Commit{Timeout: runner.DurationPtr(0)},
				Timeout:   runner.DurationPtr(60 * time.Second),
			},
			wantName: "claude",
			wantArgs: []string{},
			wantDead: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spy := &runner.DeadlineRecordingRunner{}
			transport := aitransport.New(spy, tt.cfg, tt.verb)
			if _, err := transport.Generate(t.Context(), "prompt"); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			if spy.Name() != tt.wantName {
				t.Errorf("invoked binary = %q, want the %v accessor-resolved binary %q", spy.Name(), tt.verb, tt.wantName)
			}
			if len(spy.Args()) != len(tt.wantArgs) {
				t.Fatalf("invoked args = %v, want %v", spy.Args(), tt.wantArgs)
			}
			for i := range tt.wantArgs {
				if spy.Args()[i] != tt.wantArgs[i] {
					t.Errorf("invoked args = %v, want the %v accessor-resolved args %v", spy.Args(), tt.verb, tt.wantArgs)
				}
			}
			if spy.HadDeadline() != tt.wantDead {
				t.Errorf("context hadDeadline = %v, want %v (timeout must be sourced from cfg.TimeoutFor(%v), never zero-by-omission)", spy.HadDeadline(), tt.wantDead, tt.verb)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
