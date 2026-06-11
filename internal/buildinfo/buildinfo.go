// Package buildinfo holds mint's OWN tool version in ONE place. Both version entry
// points — the `mint version` subcommand and the `mint --version` global flag —
// read this single var, so they can never report divergent values.
//
// This is mint's TOOL version, distinct from the RELEASE version that
// internal/version derives from git tags. Printing this never touches git: it is
// the build identity of the mint binary itself, not the version being released.
//
// Version is a package-level var (not a const) so it is build-time injectable:
//
//	go build -ldflags '-X mint/internal/buildinfo.Version=1.4.0' ./cmd/mint
//
// An unstamped binary (`go run`, a plain `go build`) reports the "dev" default.
package buildinfo

// Version is mint's own tool version, defaulting to "dev" until the release build
// stamps the real value via -ldflags -X. It is the SINGLE source both version
// surfaces read.
var Version = "dev"
