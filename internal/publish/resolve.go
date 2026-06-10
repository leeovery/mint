package publish

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"mint/internal/runner"
)

// ErrProviderUnresolved is the distinguishable outcome when no publishing driver
// can be selected: a non-recognised host with no override, a recognised-but-
// unsupported provider value, or no remote URL to detect from. ResolvePublisher
// returns it (with a nil Publisher) rather than guessing a driver, so the run
// never silently assumes GitHub. The safe-downgrade-to-tag+push behaviour layered
// on top of this sentinel (task 4-10) warns and downgrades; callers branch on it
// with errors.Is, and read the human-readable cause via *UnresolvedError.Reason()
// (errors.As).
var ErrProviderUnresolved = errors.New("publish: provider could not be resolved")

// UnresolvedError is the concrete error ResolvePublisher returns when no driver
// can be selected. It WRAPS ErrProviderUnresolved (so errors.Is keeps matching the
// sentinel) and carries a short, human-readable reason naming WHY resolution failed
// — an unsupported provider value, an unrecognised host, no remote, or an
// unparseable remote URL — so the engine's safe-downgrade warning (task 4-10) can
// tell the user which cause downgraded the run to tag + push only.
type UnresolvedError struct {
	reason string
}

// Reason returns the short, display-ready cause of the unresolved outcome (e.g.
// `unsupported provider "gitlab"` or `unrecognised host "gitlab.com"`).
func (e *UnresolvedError) Reason() string {
	return e.reason
}

// Error renders the sentinel text with the specific reason appended.
func (e *UnresolvedError) Error() string {
	return ErrProviderUnresolved.Error() + ": " + e.reason
}

// Unwrap exposes the sentinel so errors.Is(err, ErrProviderUnresolved) matches.
func (e *UnresolvedError) Unwrap() error {
	return ErrProviderUnresolved
}

// unresolved builds an *UnresolvedError carrying a formatted reason.
func unresolved(format string, args ...any) error {
	return &UnresolvedError{reason: fmt.Sprintf(format, args...)}
}

// githubHost is the one remote host auto-detection maps to the GitHub driver.
const githubHost = "github.com"

// providerGitHub is the explicit provider config value that forces the GitHub
// driver regardless of the detected host.
const providerGitHub = "github"

// ResolvePublisher selects the publishing driver for a run from the release
// remote's URL and the optional provider config override, returning it behind the
// Publisher interface so callers never depend on a concrete driver type — a future
// GitLab/Gitea driver slots in here with no caller change.
//
// Selection precedence:
//
//   - An explicit providerConfig (a recognised provider name) WINS over detection:
//     "github" forces the GitHub driver regardless of the host. A recognised-but-
//     unsupported value (e.g. "gitlab", which mint has no driver for) does NOT fall
//     through to GitHub — it returns ErrProviderUnresolved so the value cannot
//     silently vanish.
//   - Otherwise the host is parsed from remoteURL (across the HTTPS, SCP-like SSH,
//     and ssh:// forms) and a github.com host selects the GitHub driver.
//   - Anything else — a non-github.com host, or an empty remoteURL (no remote) —
//     returns a nil Publisher with ErrProviderUnresolved.
//
// The GitHub driver is built over r so its gh commands flow through the same
// CommandRunner seam as the rest of the run.
func ResolvePublisher(remoteURL, providerConfig string, r runner.CommandRunner) (Publisher, error) {
	if providerConfig != "" {
		return resolveByProvider(providerConfig, r)
	}
	return resolveByHost(remoteURL, r)
}

// resolveByProvider honours an explicit provider override. The only recognised
// driver is GitHub; any other value is unsupported and returns the unresolved
// error (naming the value) rather than silently assuming GitHub.
func resolveByProvider(provider string, r runner.CommandRunner) (Publisher, error) {
	if provider == providerGitHub {
		return NewGitHubPublisher(r), nil
	}
	return nil, unresolved("unsupported provider %q", provider)
}

// resolveByHost auto-detects the driver from the remote host. A github.com host
// selects the GitHub driver; otherwise the outcome is unresolved with a reason
// distinguishing its three causes: no remote at all (empty URL), a remote whose
// host could not be parsed (a non-URL/unparseable SSH form), or a recognised host
// that simply has no driver.
func resolveByHost(remoteURL string, r runner.CommandRunner) (Publisher, error) {
	if remoteURL == "" {
		return nil, unresolved("no remote configured")
	}
	host := parseHost(remoteURL)
	if host == "" {
		return nil, unresolved("could not determine host from remote %q", remoteURL)
	}
	if host == githubHost {
		return NewGitHubPublisher(r), nil
	}
	return nil, unresolved("unrecognised host %q", host)
}

// parseHost extracts the bare host from a git remote URL across the three forms
// git emits, returning "" when the URL is empty or its host cannot be determined:
//
//   - HTTPS / ssh:// (any scheme://): parsed via net/url, so userinfo (git@) and
//     the port are stripped — host only.
//   - SCP-like SSH (git@github.com:owner/repo.git): no scheme, a single colon
//     separating host:path. net/url cannot parse this form, so the host is taken
//     as the segment between an optional "user@" and the first ":".
func parseHost(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	if strings.Contains(remoteURL, "://") {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return ""
		}
		return u.Hostname()
	}
	return scpHost(remoteURL)
}

// scpHost extracts the host from the SCP-like SSH form
// [user@]host:path (e.g. git@github.com:owner/repo.git): it drops an optional
// leading "user@" and returns the text up to the first ":". A form with no colon
// is not an SCP URL, so it yields "".
func scpHost(remoteURL string) string {
	if at := strings.LastIndex(remoteURL, "@"); at != -1 {
		remoteURL = remoteURL[at+1:]
	}
	host, _, found := strings.Cut(remoteURL, ":")
	if !found {
		return ""
	}
	return host
}
