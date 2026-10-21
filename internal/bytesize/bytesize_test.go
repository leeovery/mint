package bytesize_test

import (
	"testing"

	"mint/internal/bytesize"
)

func TestFormat_WritesDecimalUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int
		want  string
	}{
		{bytes: 0, want: "0 B"},
		{bytes: 640, want: "640 B"},
		{bytes: 999, want: "999 B"},
		{bytes: 1_000, want: "1.0 KB"},
		{bytes: 9_949, want: "9.9 KB"},
		{bytes: 9_950, want: "10 KB"},
		{bytes: 850_000, want: "850 KB"},
		{bytes: 999_499, want: "999 KB"},
		{bytes: 999_500, want: "1.0 MB"},
		{bytes: 1_500_000, want: "1.5 MB"},
		{bytes: 3_100_000, want: "3.1 MB"},
		{bytes: 9_950_000, want: "10 MB"},
		{bytes: 38_000_000, want: "38 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := bytesize.Format(tt.bytes); got != tt.want {
				t.Errorf("Format(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
