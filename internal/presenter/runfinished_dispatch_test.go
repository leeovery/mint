package presenter_test

import (
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// This file is the focused end-of-run DISPATCH suite: it proves RunFinished is an
// EXHAUSTIVE, suppression-first dispatch on the run's Outcome in BOTH render modes.
// The outcome-specific CONTENT (release footer form, regenerate summary form) is
// locked by the existing plain_test/pretty_test cases; here the focus is the
// dispatch table itself — the no-footer arms, the leaf-from-payload for RELEASE,
// and that suppression precedes shaping for EVERY outcome.

// TestRunFinishedReleaseFooterRendersWithURL locks the release arm's footer WITH
// the url in both modes — plain "done: {project} v{X} {url}", pretty
// "{leaf} released {project} v{X} · {url}".
func TestRunFinishedReleaseFooterRendersWithURL(t *testing.T) {
	const url = "https://github.com/acme/acme/releases/tag/v1.4.0"

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0", URL: url})
		})

		want := "done: acme v1.4.0 " + url + "\n"
		if got := out.String(); got != want {
			t.Errorf("plain release footer = %q, want %q", got, want)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0", URL: url})
		})

		want := "\n🌿 released acme v1.4.0 · " + url + "\n"
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
		{name: "supplied leaf used verbatim", leaf: "🌱", want: "\n🌱 released acme v1.4.0\n"},
		{name: "empty leaf defaults to mint leaf", leaf: "", want: "\n🌿 released acme v1.4.0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0", Leaf: tt.leaf})
			})

			if got := out.String(); got != tt.want {
				t.Errorf("release footer leaf %q = %q, want %q", tt.leaf, got, tt.want)
			}
		})
	}
}

// TestRunFinishedRegenerateCloseRendersWithoutURL locks the regenerate arm's
// URL-less close in both modes — no url, no dangling separator. The engine sets
// Outcome=OutcomeAllRegenerated and the Summary; the URL field is omitted entirely.
func TestRunFinishedRegenerateCloseRendersWithoutURL(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeAllRegenerated, Project: "acme", Summary: "v1.4.0"})
		})

		if got := out.String(); got != "done: acme v1.4.0\n" {
			t.Errorf("plain regenerate close = %q, want exactly %q (no url, no dangling separator)", got, "done: acme v1.4.0\n")
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeAllRegenerated, Project: "acme", Summary: "v1.4.0"})
		})

		if got := out.String(); got != "\n🌿 regenerated acme v1.4.0\n" {
			t.Errorf("pretty regenerate close = %q, want exactly %q (no url, no dangling separator)", got, "\n🌿 regenerated acme v1.4.0\n")
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
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeCommitted, Project: "acme"})
		})

		if got := out.String(); got != "done: acme committed\n" {
			t.Errorf("plain commit close = %q, want exactly %q (no version, no url)", got, "done: acme committed\n")
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeCommitted, Project: "acme"})
		})

		if got := out.String(); got != "\n🌿 committed acme\n" {
			t.Errorf("pretty commit close = %q, want exactly %q (committed, not released; no v segment)", got, "\n🌿 committed acme\n")
		}
	})
}

// TestRunFinishedQuietOutcomesRenderNoFooter proves the no-footer arms: a single
// regenerated version never had a close line, and a declined, unwound or failed run
// ends on its own lines, so RunFinished renders NOTHING for those outcomes in both
// modes. A URL, Version or Summary on the payload must NOT coax a footer out.
func TestRunFinishedQuietOutcomesRenderNoFooter(t *testing.T) {
	outcomes := []struct {
		name    string
		outcome presenter.Outcome
	}{
		{"regenerated", presenter.OutcomeRegenerated},
		{"declined", presenter.OutcomeDeclined},
		{"unwound", presenter.OutcomeUnwound},
		{"failed", presenter.OutcomeFailed},
	}

	for _, o := range outcomes {
		result := presenter.RunResult{Outcome: o.outcome, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0", Summary: "v1.4.0"}

		t.Run("plain/"+o.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) { p.RunFinished(result) })

			if got := out.String(); got != "" {
				t.Errorf("plain %s must render NO footer, got %q", o.name, got)
			}
		})

		t.Run("pretty/"+o.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) { p.RunFinished(result) })

			if got := out.String(); got != "" {
				t.Errorf("pretty %s must render NO footer, got %q", o.name, got)
			}
		})
	}
}

// TestRunFinishedDryRunRendersItsPreviewLine locks the dry run's close in both
// modes: it names the version and says nothing changed, never "released".
func TestRunFinishedDryRunRendersItsPreviewLine(t *testing.T) {
	result := presenter.RunResult{Outcome: presenter.OutcomeDryRun, Project: "acme", Version: "1.4.0"}

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) { p.RunFinished(result) })

		if got, want := out.String(), "dry run: acme v1.4.0 — no changes made\n"; got != want {
			t.Errorf("plain dry-run close = %q, want %q", got, want)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) { p.RunFinished(result) })

		if got, want := out.String(), "\n🌿 dry run · acme v1.4.0 — no changes made\n"; got != want {
			t.Errorf("pretty dry-run close = %q, want %q", got, want)
		}
	})
}

// TestRunFinishedFailureSuppressesReleaseFooterDespiteURL proves suppression
// PRECEDES shaping for the RELEASE arm: a prior StageFailed sets terminalFailure,
// so RunFinished{Outcome:OutcomeReleased} with a URL emits NO footer despite carrying the
// URL — the success line is success-only.
func TestRunFinishedFailureSuppressesReleaseFooterDespiteURL(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if strings.Contains(out.String(), "done:") {
			t.Errorf("plain release footer must be suppressed after a failure despite the URL, got:\n%q", out.String())
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0"})
		})

		if strings.Contains(out.String(), "released") {
			t.Errorf("pretty release footer must be suppressed after a failure despite the URL, got:\n%q", out.String())
		}
	})
}

// TestRunFinishedAbortSuppressesRegenerateClose proves suppression precedes
// shaping for the REGENERATE arm on the ABORT path: an Unwound with no prior
// StageFailed (gate-n abort) sets terminalFailure, so RunFinished{Outcome:OutcomeAllRegenerated}
// emits NO closing summary.
func TestRunFinishedAbortSuppressesRegenerateClose(t *testing.T) {
	const summaryPlain = "removed tag v1.4.0, reset 2 commits; repo clean"
	const summaryPretty = "removed tag v1.4.0, reset 2 release commit(s) — repo clean"

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.Unwound(presenter.Unwind{Undone: summaryPlain})
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeAllRegenerated, Project: "acme", Summary: "v1.4.0"})
		})

		if strings.Contains(out.String(), "done:") {
			t.Errorf("plain regenerate close must be suppressed after an abort, got:\n%q", out.String())
		}
	})

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.Unwound(presenter.Unwind{Undone: summaryPretty})
			p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeAllRegenerated, Project: "acme", Summary: "v1.4.0"})
		})

		if strings.Contains(out.String(), "regenerated") {
			t.Errorf("pretty regenerate close must be suppressed after an abort, got:\n%q", out.String())
		}
	})
}

// TestRunFinishedSuppressionPrecedesShapingForEveryOutcome locks the load-bearing
// dispatch ordering: suppression is checked FIRST, BEFORE the outcome switch, so a
// terminal-failure run renders NOTHING for EVERY outcome — including the ones that
// carry a footer on their own.
func TestRunFinishedSuppressionPrecedesShapingForEveryOutcome(t *testing.T) {
	outcomes := []struct {
		name    string
		outcome presenter.Outcome
	}{
		{"released", presenter.OutcomeReleased},
		{"released with warning", presenter.OutcomeReleasedWithWarning},
		{"dry run", presenter.OutcomeDryRun},
		{"committed", presenter.OutcomeCommitted},
		{"committed with warning", presenter.OutcomeCommittedWithWarning},
		{"regenerated", presenter.OutcomeRegenerated},
		{"all regenerated", presenter.OutcomeAllRegenerated},
		{"declined", presenter.OutcomeDeclined},
		{"unwound", presenter.OutcomeUnwound},
		{"failed", presenter.OutcomeFailed},
	}

	for _, o := range outcomes {
		result := presenter.RunResult{Outcome: o.outcome, Project: "acme", Version: "1.4.0", URL: "https://example/v1.4.0", Summary: "v1.4.0"}

		t.Run("plain/"+o.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
				p.RunFinished(result)
			})

			if got := out.String(); strings.Contains(got, "done:") || strings.Contains(got, "dry run:") {
				t.Errorf("plain %s: terminal-failure run must emit no footer for any outcome, got:\n%q", o.name, got)
			}
		})

		t.Run("pretty/"+o.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected"})
				p.RunFinished(result)
			})

			got := out.String()
			for _, footer := range []string{"released", "regenerated", "committed", "dry run"} {
				if strings.Contains(got, footer) {
					t.Errorf("pretty %s: terminal-failure run must emit no footer for any outcome, got:\n%q", o.name, got)
				}
			}
		})
	}
}

// TestRunFinishedWithWarningStillEmitsTheFooter proves a warning past the point of
// no return does not suppress the footer (Warn does not set terminalFailure): the
// release and the commit both landed, so a Warn then a with-warning RunFinished
// still renders the success line, exactly as for a clean run.
func TestRunFinishedWithWarningStillEmitsTheFooter(t *testing.T) {
	const url = "https://example/v1.4.0"

	cases := []struct {
		name        string
		result      presenter.RunResult
		plain       string
		prettyMatch string
	}{
		{
			name:        "release",
			result:      presenter.RunResult{Outcome: presenter.OutcomeReleasedWithWarning, Project: "acme", Version: "1.4.0", URL: url},
			plain:       "done: acme v1.4.0 " + url + "\n",
			prettyMatch: "released acme v1.4.0 · " + url,
		},
		{
			name:        "commit",
			result:      presenter.RunResult{Outcome: presenter.OutcomeCommittedWithWarning, Project: "acme"},
			plain:       "done: acme committed\n",
			prettyMatch: "committed acme",
		},
	}

	for _, c := range cases {
		t.Run("plain/"+c.name, func(t *testing.T) {
			t.Parallel()

			out, _ := drive(func(p *presenter.PlainPresenter) {
				p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
				p.RunFinished(c.result)
			})

			if !strings.HasSuffix(out.String(), c.plain) {
				t.Errorf("plain footer missing after a warn-only run:\n%q", out.String())
			}
		})

		t.Run("pretty/"+c.name, func(t *testing.T) {
			t.Parallel()

			out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
				p.Warn(presenter.Warning{Label: "post_release", Message: "hook failed"})
				p.RunFinished(c.result)
			})

			if !strings.Contains(out.String(), c.prettyMatch) {
				t.Errorf("pretty footer missing after a warn-only run:\n%q", out.String())
			}
		})
	}
}
