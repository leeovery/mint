package fsutil_test

import (
	"testing"

	"mint/internal/fsutil"
)

func TestHomeRelative(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, path, home, want string
	}{
		{"below home", "/Users/lee/Code/pigeon", "/Users/lee", "~/Code/pigeon"},
		{"home itself", "/Users/lee", "/Users/lee", "~"},
		{"outside home", "/srv/pigeon", "/Users/lee", "/srv/pigeon"},
		{"a sibling sharing the prefix", "/Users/leeovery/pigeon", "/Users/lee", "/Users/leeovery/pigeon"},
		{"no home", "/Users/lee/Code/pigeon", "", "/Users/lee/Code/pigeon"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := fsutil.HomeRelative(tc.path, tc.home); got != tc.want {
				t.Errorf("HomeRelative(%q, %q) = %q, want %q", tc.path, tc.home, got, tc.want)
			}
		})
	}
}
