package notes_test

import (
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// The Assembler's diff and both Change Map reads carry cwd-relative pathspecs (`-- .`
// and the :(exclude) globs, recorded root-relative), so each one runs at the repo root
// the Assembler was built over, whatever directory mint started in.
func TestAssembler_GitReads_RunAtRepoRoot(t *testing.T) {
	t.Parallel()

	const root = "/work/repo"
	exclude := notes.ExcludeConfig{Globs: []string{"sub/tests/"}}

	tests := []struct {
		name     string
		read     func(a *notes.Assembler) error
		wantArgv [][]string
	}{
		{
			name: "forward diff",
			read: func(a *notes.Assembler) error {
				_, err := a.AssembleDiff(t.Context(), "v1.0.0")
				return err
			},
			wantArgv: [][]string{wantArgsWithExcludes("v1.0.0", "sub/tests/")},
		},
		{
			name: "range diff",
			read: func(a *notes.Assembler) error {
				_, err := a.AssembleRange(t.Context(), "v1.3.0..v1.4.0")
				return err
			},
			wantArgv: [][]string{append(wantRangeArgs("v1.3.0..v1.4.0"), ":(exclude)sub/tests/")},
		},
		{
			name: "forward change map",
			read: func(a *notes.Assembler) error {
				_, err := a.BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
				return err
			},
			wantArgv: changeMapArgv("v1.0.0..HEAD", "sub/tests/"),
		},
		{
			name: "range change map",
			read: func(a *notes.Assembler) error {
				_, err := a.BuildChangeMapForRange(t.Context(), "v1.3.0..v1.4.0", config.DefaultMaxPromptBytes)
				return err
			},
			wantArgv: changeMapArgv("v1.3.0..v1.4.0", "sub/tests/"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{}, nil)

			if err := tc.read(notes.NewAssembler(r, root, exclude)); err != nil {
				t.Fatalf("read returned unexpected error: %v", err)
			}

			invs := r.Invocations()
			if len(invs) != len(tc.wantArgv) {
				t.Fatalf("invocations = %d, want %d", len(invs), len(tc.wantArgv))
			}
			for i, inv := range invs {
				if inv.Dir != root {
					t.Errorf("invocation %d Dir = %q, want %q (argv %v)", i, inv.Dir, root, inv.Args)
				}
				if inv.Name != "git" {
					t.Errorf("invocation %d command = %q, want git", i, inv.Name)
				}
				if !slices.Equal(inv.Args, tc.wantArgv[i]) {
					t.Errorf("invocation %d argv = %v, want %v", i, inv.Args, tc.wantArgv[i])
				}
			}
		})
	}
}

// changeMapArgv is the two Change Map reads' exact argv over diffRange, with the
// built-in always-excludes then the given globs.
func changeMapArgv(diffRange string, globs ...string) [][]string {
	tail := []string{"--", ".", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml"}
	for _, g := range globs {
		tail = append(tail, ":(exclude)"+g)
	}
	return [][]string{
		append([]string{"diff", "--name-status", diffRange}, tail...),
		append([]string{"diff", "--numstat", diffRange}, tail...),
	}
}
