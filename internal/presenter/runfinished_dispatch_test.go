package presenter_test

import (
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// This file is the focused end-of-run DISPATCH suite for task 4-4: it proves
// RunFinished is an EXHAUSTIVE, suppression-first verb dispatch across all four
// shapes (release, regenerate, init, version) in BOTH render modes. The
// verb-specific CONTENT (release footer form, regenerate summary form) is locked
// by the existing plain_test/pretty_test cases; here the focus is the dispatch
// table itself — the no-footer arms, the leaf-from-payload for RELEASE, and that
// suppression precedes shaping for EVERY shape.

// TestRunFinishedReleaseFooterRendersWithURL locks the release arm's footer WITH
// the url in both modes — plain "done: {project} v{X} {url}", pretty
// "{leaf} released {project} v{X} · {url}".
func TestRunFinishedReleaseFooterRendersWithURL(t *testing.T) {
	const url = "https://github.com/acme/acme/releases/tag/v1.4.0"

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: url})
		})

		want := "done: acme v1.4.0 " + url + "\n"
		if got := out.String(); got != want {
			t.Errorf("plain release footer = %q, want %q", got, want)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: url})
		})

		want := "🌿 released acme v1.4.0 · " + url + "\n"
		if got := out.String(); got != want {
			t.Errorf("pretty release footer = %q, want %q", got, want)
		}
	})
}

// TestRunFinishedReleaseFooterLeafComesFromPayload locks that the RELEASE arm's
// footer leaf is the ENGINE-SUPPLIED brand leaf (r.Leaf via leafOrDefault), not a
// hardcoded literal — so a customised commit_prefix brand stays consistent with
// the start-of-run brand line. A supplied leaf is used verbatim; an empty leaf
// defaults to 🌿. (Plain carries no brand leaf, so this is a pretty concern.)
func TestRunFinishedReleaseFooterLeafComesFromPayload(t *testing.T) {
	tests := []struct {
		name string
		leaf string
		want string
	}{
		{name: "supplied leaf used verbatim", leaf: "🌱", want: "🌱 released acme v1.4.0\n"},
		{name: "empty leaf defaults to mint leaf", leaf: "", want: "🌿 released acme v1.4.0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", Leaf: tt.leaf})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("release footer leaf %q = %q, want %q", tt.leaf, got, tt.want)
			}
		})
	}
}

// TestRunFinishedRegenerateCloseRendersWithoutURL locks the regenerate arm's
// URL-less close in both modes — no url, no dangling separator. The engine sets
// Verb=VerbRegenerate and the Summary; the URL field is omitted entirely.
func TestRunFinishedRegenerateCloseRendersWithoutURL(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRegenerate, Project: "acme", Summary: "v1.4.0"})
		})

		if got := out.String(); got != "done: acme v1.4.0\n" {
			t.Errorf("plain regenerate close = %q, want exactly %q (no url, no dangling separator)", got, "done: acme v1.4.0\n")
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRegenerate, Project: "acme", Summary: "v1.4.0"})
		})

		if got := out.String(); got != "🌿 regenerated acme v1.4.0\n" {
			t.Errorf("pretty regenerate close = %q, want exactly %q (no url, no dangling separator)", got, "🌿 regenerated acme v1.4.0\n")
		}
	})
}

// TestRunFinishedCommitCloseRendersVersionless locks the commit arm's close in both
// modes: version-less and URL-less — plain "done: {project} committed", pretty
// "{leaf} committed {project}". A Version/URL on the payload must NOT leak in: a
// commit publishes no release and announces no version, so the arm renders neither
// segment (and crucially never the release arm's "released … v" with its dangling
// empty version).
func TestRunFinishedCommitCloseRendersVersionless(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbCommit, Project: "acme"})
		})

		if got := out.String(); got != "done: acme committed\n" {
			t.Errorf("plain commit close = %q, want exactly %q (no version, no url)", got, "done: acme committed\n")
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbCommit, Project: "acme"})
		})

		if got := out.String(); got != "🌿 committed acme\n" {
			t.Errorf("pretty commit close = %q, want exactly %q (committed, not released; no v segment)", got, "🌿 committed acme\n")
		}
	})
}

// TestRunFinishedInitRendersNoFooter proves the VerbInit no-footer arm: init has
// no release-style footer, so RunFinished renders NOTHING for VerbInit in both
// modes (defensive completeness — in practice the engine never calls RunFinished
// for init). A URL/Version on the payload must NOT coax a footer out.
func TestRunFinishedInitRendersNoFooter(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbInit, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if got := out.String(); got != "" {
			t.Errorf("plain VerbInit must render NO footer, got %q", got)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbInit, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if got := out.String(); got != "" {
			t.Errorf("pretty VerbInit must render NO footer, got %q", got)
		}
	})
}

// TestRunFinishedVersionRendersNoFooter proves the VerbVersion no-footer arm:
// version's value line IS its terminal output, so RunFinished renders NOTHING for
// VerbVersion in both modes (defensive completeness — in practice the engine never
// calls RunFinished for version).
func TestRunFinishedVersionRendersNoFooter(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbVersion, Project: "acme", Version: "1.4.0"})
		})

		if got := out.String(); got != "" {
			t.Errorf("plain VerbVersion must render NO footer, got %q", got)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbVersion, Project: "acme", Version: "1.4.0"})
		})

		if got := out.String(); got != "" {
			t.Errorf("pretty VerbVersion must render NO footer, got %q", got)
		}
	})
}

// TestRunFinishedFailureSuppressesReleaseFooterDespiteURL proves suppression
// PRECEDES shaping for the RELEASE arm: a prior StageFailed sets terminalFailure,
// so RunFinished{Verb:VerbRelease} with a URL emits NO footer despite carrying the
// URL — the success line is success-only.
func TestRunFinishedFailureSuppressesReleaseFooterDespiteURL(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if strings.Contains(out.String(), "done:") {
			t.Errorf("plain release footer must be suppressed after a failure despite the URL, got:\n%q", out.String())
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if strings.Contains(out.String(), "released") {
			t.Errorf("pretty release footer must be suppressed after a failure despite the URL, got:\n%q", out.String())
		}
	})
}

// TestRunFinishedAbortSuppressesRegenerateClose proves suppression precedes
// shaping for the REGENERATE arm on the ABORT path: an Unwound with no prior
// StageFailed (gate-n abort) sets terminalFailure, so RunFinished{Verb:VerbRegenerate}
// emits NO closing summary.
func TestRunFinishedAbortSuppressesRegenerateClose(t *testing.T) {
	const summaryPlain = "removed tag v1.4.0, reset 2 commits; repo clean"
	const summaryPretty = "removed tag v1.4.0, reset 2 release commit(s) — repo clean"

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.Unwound(presenter.Unwind{Summary: summaryPlain})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRegenerate, Project: "acme", Summary: "v1.4.0"})
		})

		if strings.Contains(out.String(), "done:") {
			t.Errorf("plain regenerate close must be suppressed after an abort, got:\n%q", out.String())
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.Unwound(presenter.Unwind{Summary: summaryPretty})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRegenerate, Project: "acme", Summary: "v1.4.0"})
		})

		if strings.Contains(out.String(), "regenerated") {
			t.Errorf("pretty regenerate close must be suppressed after an abort, got:\n%q", out.String())
		}
	})
}

// TestRunFinishedSuppressionPrecedesShapingForEveryVerb locks the load-bearing
// dispatch ordering: suppression is checked FIRST, BEFORE the verb switch, so a
// terminal-failure run renders NOTHING for EVERY shape — including the no-footer
// init/version arms (trivially nothing, but the point is the suppression branch is
// reached first for all shapes, not the verb arm). This pins "suppression precedes
// shaping" across the whole table.
func TestRunFinishedSuppressionPrecedesShapingForEveryVerb(t *testing.T) {
	verbs := []struct {
		name string
		verb presenter.RunVerb
	}{
		{"release", presenter.VerbRelease},
		{"regenerate", presenter.VerbRegenerate},
		{"init", presenter.VerbInit},
		{"version", presenter.VerbVersion},
		{"commit", presenter.VerbCommit},
	}

	for _, v := range verbs {
		t.Run("plain/"+v.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
				p.RunFinished(presenter.RunResult{Verb: v.verb, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0", Summary: "v1.4.0"})
			})

			if strings.Contains(out.String(), "done:") {
				t.Errorf("plain %s: terminal-failure run must emit no footer for any shape, got:\n%q", v.name, out.String())
			}
		})

		t.Run("pretty/"+v.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
				p.RunFinished(presenter.RunResult{Verb: v.verb, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0", Summary: "v1.4.0"})
			})

			got := out.String()
			if strings.Contains(got, "released") || strings.Contains(got, "regenerated") || strings.Contains(got, "committed") {
				t.Errorf("pretty %s: terminal-failure run must emit no footer for any shape, got:\n%q", v.name, got)
			}
		})
	}
}

// TestRunFinishedWarnOnlyStillEmitsVerbFooter proves Warn ALONE does not suppress
// the footer (Warn does not set terminalFailure): a Warn then a successful release
// RunFinished still emits the verb-shaped footer. This reuses the 2-6/2-8
// semantics — only StageFailed/Unwound set terminalFailure.
func TestRunFinishedWarnOnlyStillEmitsVerbFooter(t *testing.T) {
	const url = "https://example/v1.4.0"

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: url})
		})

		want := "done: acme v1.4.0 " + url + "\n"
		if !strings.Contains(out.String(), want) {
			t.Errorf("plain release footer missing after a warn-only run:\n%q", out.String())
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
			p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: "acme", Version: "1.4.0", URL: url})
		})

		if !strings.Contains(out.String(), "released acme v1.4.0 · "+url) {
			t.Errorf("pretty release footer missing after a warn-only run:\n%q", out.String())
		}
	})
}
