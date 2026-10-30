package rail_test

import "mint/internal/presenter/rail"

// The screens the design doc draws, pigeon going v1.3.2 → v1.4.0, as models: the
// mock-ups' wording, but for mint's own where the doc gives it.

const (
	savedPath = "~/Library/Caches/mint/chunk-notes/Users-lee-Code-pigeon/v1.4.0.md"
	notesBody = "Webhooks retry on their own now, so a flaky endpoint no longer drops events.\n" +
		"\n" +
		"✨ Added\n" +
		"- Failed webhook deliveries retry with backoff for up to an hour.\n" +
		"- `pigeon replay <id>` resends any delivery from the last 30 days.\n" +
		"\n" +
		"🐛 Fixed\n" +
		"- The dashboard no longer shows a delivery twice after a reconnect.\n" +
		"- Signing secrets with a trailing newline are accepted again.\n"
)

func releaseHeader() rail.Header {
	return rail.Header{Command: "release", Where: "~/Code/pigeon", Version: "0.1.2"}
}

func releaseStart() rail.Station {
	return rail.Station{Kind: rail.Start, Name: "v1.3.2", Detail: "main @ 7c1e0b4"}
}

// prepared is a release up to its notes.
func prepared() []rail.Item {
	return []rail.Item{
		releaseHeader(),
		releaseStart(),
		rail.Node{Name: "version", Detail: "v1.4.0 (minor bump)"},
		rail.Node{Name: "preflight", Detail: "tree clean · on main · v1.4.0 free · in sync with origin"},
		rail.Node{Name: "pre_tag", Detail: "npm ci && npm run build", Time: "4.1s", Lines: []rail.Hang{
			{Kind: rail.Note, Text: "committed dist/ (3 files) as chore(release): pre-tag artifacts"},
		}},
		rail.Node{Name: "check", Detail: "files that don't belong in the notes", Time: "6.2s", Lines: []rail.Hang{
			{Kind: rail.Note, Text: "docs/specs/** excluded · design documents"},
			{Kind: rail.Note, Text: "written to .mint.toml's [release] table and committed"},
		}},
	}
}

// ahead is the release's plan from the record on, the end station waiting.
func ahead() []rail.Live {
	return []rail.Live{
		rail.Node{State: rail.ToCome, Name: "record", Detail: "CHANGELOG.md · package.json"},
		rail.Rule{},
		rail.Node{State: rail.ToCome, Name: "push", Detail: "main + v1.4.0 → origin, atomic"},
		rail.Node{State: rail.ToCome, Name: "publish", Detail: "GitHub release"},
		rail.Node{State: rail.ToCome, Name: "post_release", Detail: "./scripts/notify.sh"},
		rail.Station{Kind: rail.Pending, Name: "v1.4.0"},
	}
}

// paused is the release at the exclusion check's second question, its first
// answered: the questions sit inside the check, and the notes wait to come.
func paused() rail.Run {
	items := prepared()
	check := rail.Node{Name: "check", Detail: "2 flagged", Time: "6.2s", Lines: []rail.Hang{
		{Kind: rail.Note, Text: "docs/specs/** excluded · design documents"},
	}}
	question := rail.Question{
		Under: true, Text: "Exclude fixtures/ from the notes?", About: "test fixtures · 14 files, 212 kB",
		Choices: []rail.Choice{{Key: "e", Action: "exclude"}, {Key: "k", Action: "keep", Default: true}},
	}
	notes := rail.Node{State: rail.ToCome, Name: "notes", Detail: "written by the AI"}
	return rail.Run{Settled: items[:len(items)-1], Region: append([]rail.Live{check, question, notes}, ahead()...)}
}

func running() rail.Run {
	notes := rail.Node{
		State: rail.Running, Name: "notes", Time: "38s", Frame: 2,
		Progress: &rail.Progress{Label: "3 of 4 parts done", Done: 3, Total: 4},
	}
	return rail.Run{Settled: prepared(), Region: append([]rail.Live{notes}, ahead()...)}
}

// review is the release at its review: the notes node the question asks about sits
// in the region with the body, as it settles with its answer.
func review() rail.Run {
	notes := rail.Node{Name: "notes", Detail: "written in 4 parts and merged", Time: "52.3s"}
	question := rail.Question{Text: "Release with these notes?", Choices: []rail.Choice{
		{Key: "y", Action: "release", Default: true}, {Key: "n", Action: "abort"}, {Key: "e", Action: "edit"}, {Key: "r", Action: "regenerate"},
	}}
	return rail.Run{Settled: prepared(), Region: append([]rail.Live{notes, rail.Body{Text: notesBody, Markdown: true}, question}, ahead()...)}
}

func done() rail.Run {
	return rail.Run{Settled: append(prepared(),
		rail.Node{Name: "notes", Detail: "written in 4 parts and merged", Tail: "accepted", Time: "52.3s"},
		rail.Node{Name: "record", Detail: "CHANGELOG.md · package.json → 🌿 Release v1.4.0"},
		rail.Rule{Crossed: true},
		rail.Node{Name: "push", Detail: "main + v1.4.0 → origin, atomic", Time: "1.9s"},
		rail.Node{Name: "publish", Detail: "GitHub release", Time: "1.3s"},
		rail.Node{Name: "post_release", Detail: "./scripts/notify.sh", Time: "0.8s"},
		rail.End{Landed: true, Name: "v1.4.0", Detail: "released", Time: "1m 14s", URL: "https://github.com/acme/pigeon/releases/tag/v1.4.0"},
	)}
}

func failed() rail.Run {
	return rail.Run{Settled: []rail.Item{
		releaseHeader(),
		releaseStart(),
		rail.Node{Name: "version", Detail: "v1.4.0 (minor bump)", Tail: "accepted (-y)"},
		rail.Node{Name: "preflight", Detail: "tree clean · on main · v1.4.0 free · in sync with origin"},
		rail.Node{Name: "pre_tag", Detail: "npm ci && npm run build", Time: "4.1s", Lines: []rail.Hang{
			{Kind: rail.Note, Text: "committed dist/ (3 files) as chore(release): pre-tag artifacts"},
		}},
		rail.Node{Name: "check", Detail: "files that don't belong in the notes", Time: "6.2s"},
		rail.Node{State: rail.Warning, Name: "docs/specs/**", Detail: "flagged, not asked: design documents · 6 files, 48 kB"},
		rail.Node{State: rail.Failed, Name: "notes", Detail: "the AI call for part 3 of 4 failed", Time: "41.0s", Lines: []rail.Hang{
			{Kind: rail.Output, Text: "Claude usage limit reached. Your limit will reset at 4pm (Europe/London).\n"},
			{Kind: rail.Note, Text: "part notes saved to"},
			{Kind: rail.Path, Text: savedPath},
		}},
		rail.Node{State: rail.Unwind, Name: "unwind", Detail: "pre-tag artifacts commit removed"},
		rail.End{Name: "v1.3.2", Detail: "back where it started · nothing pushed", Time: "47.0s"},
		rail.Next{Moves: []rail.Move{
			{Lead: "Tidy the saved part notes, then release with them:", Command: "mint release -m --notes-file " + savedPath},
			{Lead: "Release with the commit-subject list:", Command: "mint release -m --no-ai"},
			{Lead: "Ship that list whenever the AI fails, under [release] in .mint.toml:", Command: `on_notes_failure = "fallback"`},
			{Lead: "Leave docs/specs/** out of the notes, under [release] in .mint.toml:", Command: `diff_exclude = ["docs/specs/**"]`},
		}},
	}}
}

func commit() rail.Run {
	return rail.Run{Settled: []rail.Item{
		rail.Header{Command: "commit", Where: "~/Code/pigeon", Version: "0.1.2"},
		rail.Station{Kind: rail.Start, Name: "main", Detail: "7c1e0b4"},
		rail.Node{Name: "diff", Detail: "4 files · +86 −12 · everything, staged when you accept"},
		rail.Node{Name: "message", Detail: "written", Tail: "accepted", Time: "2.1s"},
		rail.Node{Name: "stage", Detail: "4 files"},
		rail.Node{Name: "commit", Detail: "a3f9c21"},
		rail.Node{Name: "push", Detail: "main → origin", Time: "1.2s"},
		rail.End{Landed: true, Name: "main", Detail: "a3f9c21 · pushed", Time: "4.8s"},
	}}
}

func regenerated() rail.Run {
	items := []rail.Item{
		rail.Header{Command: "release regenerate", Where: "~/Code/pigeon", Version: "0.1.2"},
		rail.Station{Kind: rail.Start, Name: "7 versions", Detail: "fresh notes → GitHub releases, then CHANGELOG.md"},
		rail.Node{Name: "v1.0.0", Detail: "GitHub release · Initial release.", Time: "0.4s"},
		rail.Node{Name: "v1.1.0", Detail: "GitHub release", Time: "11.2s"},
		rail.Node{State: rail.Warning, Name: "v1.2.0", Detail: "skipped · the AI returned empty notes", Time: "7.0s"},
	}
	for _, v := range []struct{ tag, time string }{{"v1.2.1", "5.1s"}, {"v1.3.0", "14.6s"}, {"v1.3.2", "6.3s"}, {"v1.4.0", "12.9s"}} {
		items = append(items, rail.Node{Name: v.tag, Detail: "GitHub release", Time: v.time})
	}
	items = append(items,
		rail.Node{Name: "CHANGELOG.md", Detail: "rebuilt, 7 entries · committed and pushed", Time: "2.4s"},
		rail.End{Landed: true, Name: "done", Detail: "6 of 7 versions regenerated · 1 skipped", Time: "1m 07s"},
		rail.Next{Moves: []rail.Move{
			{Lead: "v1.2.0 kept its old notes. To try it again:", Command: "mint release regenerate v1.2.0 --target both"},
		}},
	)
	return rail.Run{Settled: items}
}

// dryRun is a release's dry run: the review it would stop at, previewed with no
// keys, and the stages it skipped left hollow above its end.
func dryRun() rail.Run {
	return rail.Run{Settled: append(prepared(),
		rail.Node{Name: "notes", Detail: "written in 4 parts and merged", Time: "52.3s"},
		rail.Question{Text: "Release with these notes?", Tail: "skipped (dry run)"},
		rail.Node{State: rail.ToCome, Name: "record", Detail: "CHANGELOG.md · package.json"},
		rail.Rule{},
		rail.Node{State: rail.ToCome, Name: "push", Detail: "main + v1.4.0 → origin, atomic"},
		rail.Node{State: rail.ToCome, Name: "publish", Detail: "GitHub release"},
		rail.Node{State: rail.ToCome, Name: "post_release", Detail: "./scripts/notify.sh"},
		rail.End{Name: "v1.4.0", Detail: "dry run · nothing changed", Time: "1m 01s"},
	)}
}

// initialised is init: no stations, so no rail, a row per file.
func initialised() rail.Run {
	return rail.Run{Settled: []rail.Item{
		rail.Header{Command: "init", Where: "~/Code/pigeon", Version: "0.1.2"},
		rail.Node{Name: ".mint.toml", Detail: "created"},
		rail.Node{State: rail.LeftAlone, Name: "release", Detail: "already there · --force overwrites it"},
		rail.Node{Name: ".github/workflows/release.yml", Detail: "created"},
	}}
}
