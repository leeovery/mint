package ai_test

import (
	"testing"

	"mint/internal/ai"
)

// TestGenerationErrorOutput pins the composition rule: a stream counts only when
// non-blank after trimming, stdout before stderr, joined by one newline, interior content
// verbatim, trailing whitespace trimmed.
func TestGenerationErrorOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
		stderr string
		want   string
	}{
		{name: "stdout only", stdout: "Prompt is too long\n", want: "Prompt is too long"},
		{name: "stderr only", stderr: "Credit balance is too low\n", want: "Credit balance is too low"},
		{
			name:   "both streams join stdout first with one newline",
			stdout: "Prompt is too long\n",
			stderr: "warning: retrying\n",
			want:   "Prompt is too long\n\nwarning: retrying",
		},
		{
			name:   "whitespace-only stdout counts as empty",
			stdout: " \n\t\n",
			stderr: "Credit balance is too low\n",
			want:   "Credit balance is too low",
		},
		{name: "two whitespace-only streams compose nothing", stdout: "  \n", stderr: "\t\n", want: ""},
		{name: "no streams compose nothing", want: ""},
		{
			name:   "interior whitespace kept verbatim",
			stdout: "  line one\n\n  line two  \n\n",
			want:   "  line one\n\n  line two",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			genErr := &ai.GenerationError{Stdout: tt.stdout, Stderr: tt.stderr, ExitCode: 1}

			if got := genErr.Output(); got != tt.want {
				t.Errorf("Output() = %q, want %q", got, tt.want)
			}
			if genErr.Stdout != tt.stdout || genErr.Stderr != tt.stderr {
				t.Errorf("Output() mutated the raw streams: stdout %q stderr %q", genErr.Stdout, genErr.Stderr)
			}
		})
	}
}
