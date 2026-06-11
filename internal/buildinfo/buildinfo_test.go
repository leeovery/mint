package buildinfo_test

import (
	"testing"

	"mint/internal/buildinfo"
)

// TestVersionDefaultsToDev proves the single tool-version source carries the
// untagged "dev" default when no build-time injection occurred. This is the value
// a `go run` / unstamped binary reports.
func TestVersionDefaultsToDev(t *testing.T) {
	if buildinfo.Version != "dev" {
		t.Errorf("buildinfo.Version = %q, want %q (the unstamped default)", buildinfo.Version, "dev")
	}
}

// TestVersionIsAPackageVarOverridable proves Version is a writable package-level
// var — the requirement that lets `-ldflags '-X mint/internal/buildinfo.Version=<v>'`
// stamp the real version at build time (and lets a test pin a known value). A const
// would not compile against this assignment, so this test is the compile-time guard
// that Version stays an injectable var.
func TestVersionIsAPackageVarOverridable(t *testing.T) {
	original := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = original })

	buildinfo.Version = "9.9.9"
	if buildinfo.Version != "9.9.9" {
		t.Errorf("after override buildinfo.Version = %q, want %q", buildinfo.Version, "9.9.9")
	}
}
