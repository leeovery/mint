package main

import (
	"reflect"
	"testing"
)

// TestClassifyCommand verifies the top-level dispatch routing: `release` with no
// subcommand is the cut action, `release regenerate ...` routes to the regenerate
// subcommand (a subcommand of release, NOT a top-level verb), and anything else
// is an unknown command. The classifier is pure — it makes no execution decision,
// only resolves which route the args select.
func TestClassifyCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		wantKind commandKind
		wantRest []string
	}{
		{
			name:     "bare release is the cut action",
			args:     []string{"release", "-m"},
			wantKind: commandRelease,
			wantRest: []string{"-m"},
		},
		{
			name:     "release regenerate routes to the regenerate subcommand",
			args:     []string{"release", "regenerate", "1.4.0", "--reuse"},
			wantKind: commandRegenerate,
			wantRest: []string{"1.4.0", "--reuse"},
		},
		{
			name:     "bare regenerate is unknown (not a top-level command)",
			args:     []string{"regenerate", "1.4.0"},
			wantKind: commandUnknown,
		},
		{
			name:     "no args is unknown",
			args:     nil,
			wantKind: commandUnknown,
		},
		{
			name:     "release with no subcommand is the cut action",
			args:     []string{"release"},
			wantKind: commandRelease,
			wantRest: []string{},
		},
		{
			name:     "init is a top-level verb",
			args:     []string{"init"},
			wantKind: commandInit,
			wantRest: []string{},
		},
		{
			name:     "init carries its flags through",
			args:     []string{"init", "--force"},
			wantKind: commandInit,
			wantRest: []string{"--force"},
		},
		{
			name:     "version is a top-level verb",
			args:     []string{"version"},
			wantKind: commandVersion,
			wantRest: []string{},
		},
		{
			name:     "commit is a top-level verb",
			args:     []string{"commit"},
			wantKind: commandCommit,
			wantRest: []string{},
		},
		{
			name:     "commit carries its flags through",
			args:     []string{"commit", "--plain"},
			wantKind: commandCommit,
			wantRest: []string{"--plain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kind, rest := classifyCommand(tt.args)
			if kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", kind, tt.wantKind)
			}
			if tt.wantKind != commandUnknown && !reflect.DeepEqual(rest, tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
		})
	}
}
