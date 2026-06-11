// This file is the engine's `mint init` orchestrator (task 6-7): it activates mint
// in a project by dropping in the two scaffolding files — `.mint.toml` and the
// `release` shim — at the git-resolved repo root, idempotently and non-clobbering,
// with `--force` to regenerate.
//
// Init owns ONLY the orchestration: it resolves the root via the CommandRunner
// (gitrepo.ResolveRoot → `git rev-parse --show-toplevel`), reads the static content
// from the pure initgen generators, and emits one InitResult per target through the
// presenter seam. The CONTENT lives in initgen (a pure string generator with no IO);
// the RENDERING lives in the presenter (which owns the created/skipped line shapes).
// The engine's job here is the disposition: for each target it resolves
// created-vs-skipped (existence + --force) and supplies the short skip reason, so the
// presenter never inspects --force.
//
// init has NO interactive gate (its safety is structural — non-clobber + --force, not
// a prompt) and NO release-style end-of-run footer: the per-file created/skipped
// InitResult lines ARE the terminal output, so Init never calls Prompt or
// RunFinished.

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mint/internal/gitrepo"
	"mint/internal/initgen"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// skipReason is the SHORT, engine-supplied explanation attached to an InitSkipped
// outcome — not a pre-formatted sentence. The presenter owns the line shape (e.g.
// plain "{target}: skipped (exists, use --force)"); the engine supplies only this
// reason fragment, rendered verbatim.
const skipReason = "exists, use --force"

// InitDeps is the minimal seam set the init orchestrator needs: the Presenter it
// narrates each outcome through and the read-only Runner it resolves the repo root
// with. It is deliberately NARROWER than ReleaseDeps — init writes two static files
// and runs no git mutations, AI, or publisher — so it depends only on what it uses
// (interface segregation), keeping init decoupled from the release pipeline's heavy
// dependency graph.
type InitDeps struct {
	// Presenter is mint's single output seam. Init emits one InitResult per target
	// and nothing else — no Prompt, no RunFinished.
	Presenter presenter.Presenter
	// Runner is the read-only external-command seam used solely to resolve the repo
	// root via `git rev-parse --show-toplevel`. Init writes files directly (no git
	// mutation), so no lock-resilient Mutator is needed.
	Runner runner.CommandRunner
}

// InitOptions is the init run's resolved option set. Force mirrors the `--force`
// flag: when true a target is written even if it already exists (the regenerate
// path); when false an existing target is skipped, leaving it byte-for-byte
// unchanged.
type InitOptions struct {
	// Force overwrites an existing target instead of skipping it. The engine resolves
	// the resulting disposition (InitCreated under force, InitSkipped otherwise) — the
	// presenter never sees this flag.
	Force bool
}

// initTarget is one scaffolding file Init drops at the repo root: the relative Name
// rendered verbatim in the InitResult (e.g. ".mint.toml"), the static Content from
// the pure generator, and the file Mode it is written with (executable for the
// shim, ordinary for the config).
type initTarget struct {
	// Name is the target's path relative to the repo root, used both to build the
	// absolute write path and as the InitResult Target rendered to the user.
	Name string
	// Content is the static file body from the initgen generator.
	Content string
	// Mode is the file mode the target is written with — executable (ShimMode) for
	// the shim so `./release` runs straight after init, ordinary for the config.
	Mode os.FileMode
}

// configFileName / shimFileName are the two relative target names init drops at the
// repo root.
const (
	configFileName = ".mint.toml"
	shimFileName   = "release"
)

// configFileMode is the ordinary (non-executable) mode the `.mint.toml` template is
// written with; the shim uses initgen.ShimMode instead.
const configFileMode os.FileMode = 0o644

// Init activates mint in a project: it resolves the repo root via the CommandRunner
// and writes the two scaffolding files there — `.mint.toml` (the commented config
// template) and `release` (the executable shim) — idempotently. For each target it
// resolves the disposition and narrates it through the presenter:
//
//   - The file does NOT exist → write it → InitResult(InitCreated).
//   - It EXISTS and --force is NOT set → skip (leave it untouched) →
//     InitResult(InitSkipped, reason).
//   - --force IS set → write/overwrite regardless → InitResult(InitCreated).
//
// The two targets are INDEPENDENT — one existing does not block the other. Files
// land at the git-resolved root, never the invocation directory. Init scaffolds
// NOTHING else (no hook scripts, prompt-override file, or auto-detected content) and
// calls neither Prompt nor RunFinished — the created/skipped lines are the terminal
// output.
//
// A root-resolution failure (not a git work tree) aborts BEFORE any target is
// considered, so a non-repo invocation writes nothing and emits no outcome.
func Init(ctx context.Context, deps InitDeps, opts InitOptions) error {
	root, err := gitrepo.ResolveRoot(ctx, deps.Runner)
	if err != nil {
		return fmt.Errorf("resolving repo root for init: %w", err)
	}

	targets := []initTarget{
		{Name: configFileName, Content: initgen.MintTOML(), Mode: configFileMode},
		{Name: shimFileName, Content: initgen.ReleaseShim(), Mode: initgen.ShimMode},
	}
	for _, t := range targets {
		if err := scaffoldTarget(deps.Presenter, root, t, opts.Force); err != nil {
			return err
		}
	}
	return nil
}

// scaffoldTarget resolves and applies the disposition for ONE target: it skips an
// existing file when force is off (narrating InitSkipped with the short reason and
// leaving the file untouched), otherwise writes the content at the resolved mode and
// narrates InitCreated. A --force overwrite arrives here as a plain write reported
// InitCreated — the presenter never distinguishes it from a fresh write.
//
// The mode is enforced with an explicit os.Chmod AFTER the write: os.WriteFile only
// applies its mode when CREATING a file, so a --force overwrite of an existing file
// would otherwise keep the old file's perms — leaving the shim non-executable if it
// had been chmod'd off. Chmod makes the executable-shim guarantee hold on both the
// create and the overwrite path.
func scaffoldTarget(p presenter.Presenter, root string, t initTarget, force bool) error {
	path := filepath.Join(root, t.Name)

	if !force && fileExists(path) {
		p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: t.Name, Reason: skipReason})
		return nil
	}

	if err := os.WriteFile(path, []byte(t.Content), t.Mode); err != nil {
		return fmt.Errorf("writing %s: %w", t.Name, err)
	}
	if err := os.Chmod(path, t.Mode); err != nil {
		return fmt.Errorf("setting mode on %s: %w", t.Name, err)
	}
	p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: t.Name})
	return nil
}

// fileExists reports whether path names an existing filesystem entry. A stat error
// other than not-exist (e.g. a permission failure) is treated as "exists" so the
// non-clobber default errs toward NOT overwriting — the safe direction when the
// state is uncertain.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
