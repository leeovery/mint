package notes_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// chunkTransport is the recording fake for the chunked path. It answers by what a
// prompt carries rather than by call order, so its replies hold whatever order the
// calls arrive in: a prompt carrying diff and a part framing is a chunk call and gets
// notes naming the slice's first file; any other prompt gets mergeBody.
type chunkTransport struct {
	mu        sync.Mutex
	mergeBody string
	prompts   []string
}

func (ct *chunkTransport) Generate(_ context.Context, prompt string) (string, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.prompts = append(ct.prompts, prompt)
	if isChunkPrompt(prompt) {
		return chunkNotesFor(firstPath(sliceOf(prompt))), nil
	}
	return ct.mergeBody, nil
}

func (ct *chunkTransport) chunkPrompts() []string {
	var out []string
	for _, p := range ct.prompts {
		if isChunkPrompt(p) {
			out = append(out, p)
		}
	}
	return out
}

func (ct *chunkTransport) mergePrompts() []string {
	var out []string
	for _, p := range ct.prompts {
		if !isChunkPrompt(p) && !strings.Contains(p, "diff --git ") {
			out = append(out, p)
		}
	}
	return out
}

// chunkNotesFor is the notes the fake returns for the chunk whose slice opens with path.
func chunkNotesFor(path string) string {
	return "- notes for the part opening with " + path
}

func isChunkPrompt(prompt string) bool {
	return strings.Contains(prompt, "diff --git ") && partFraming.MatchString(prompt)
}

// sliceOf returns the diff a chunk prompt carries: from its first file header up to
// the closing reminder.
func sliceOf(prompt string) string {
	start := strings.Index(prompt, "diff --git ")
	end := len(prompt) - len("\n\n"+notes.OutputReminder)
	if start < 0 || end < start {
		return ""
	}
	return prompt[start:end]
}

var fileHeaderPath = regexp.MustCompile(`(?m)^diff --git a/(\S+) b/`)

func firstPath(slice string) string {
	m := fileHeaderPath.FindStringSubmatch(slice)
	if m == nil {
		return ""
	}
	return m[1]
}

var partFraming = regexp.MustCompile(`part (\d+) of (\d+)`)

// partOf reads the part number and total a chunk prompt's framing names.
func partOf(t *testing.T, prompt string) (k, n int) {
	t.Helper()
	m := partFraming.FindStringSubmatch(prompt)
	if m == nil {
		t.Fatalf("chunk prompt names no part k of n:\n%.400s", prompt)
	}
	k, _ = strconv.Atoi(m[1])
	n, _ = strconv.Atoi(m[2])
	return k, n
}

// chunksInPartOrder returns the chunk prompts ordered by the part each names.
func chunksInPartOrder(t *testing.T, ct *chunkTransport) []string {
	t.Helper()
	chunks := ct.chunkPrompts()
	sort.SliceStable(chunks, func(i, j int) bool {
		ki, _ := partOf(t, chunks[i])
		kj, _ := partOf(t, chunks[j])
		return ki < kj
	})
	return chunks
}

// fileDiff renders one file's diff whose added lines total about size bytes.
func fileDiff(path string, size int) string {
	header := "diff --git a/" + path + " b/" + path + "\nindex 1111111..2222222 100644\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1,9 @@\n"
	line := "+" + strings.Repeat("x", 63) + "\n"
	return header + strings.Repeat(line, size/len(line))
}

type chunkFile struct {
	path string
	size int
}

// chunkScenario is one generation over a scripted diff whose files are in the order
// git emits them.
type chunkScenario struct {
	files []chunkFile
	cfg   config.Config
	root  string
}

func (s chunkScenario) diff() string {
	var b strings.Builder
	for _, f := range s.files {
		b.WriteString(fileDiff(f.path, f.size))
	}
	return b.String()
}

func (s chunkScenario) nameStatus() string {
	var b strings.Builder
	for _, f := range s.files {
		b.WriteString("M\t" + f.path + "\n")
	}
	return b.String()
}

func (s chunkScenario) numstat() string {
	var b strings.Builder
	for _, f := range s.files {
		fmt.Fprintf(&b, "%d\t0\t%s\n", f.size/64, f.path)
	}
	return b.String()
}

// changeMap builds the Change Map the generator is expected to carry, over the same
// scripted git output and cap.
func (s chunkScenario) changeMap(t *testing.T) string {
	t.Helper()
	m, err := notes.NewAssembler(seedChangeMapGit(t, s.nameStatus(), s.numstat()), "/repo", notes.ExcludeConfig{}).
		BuildChangeMap(t.Context(), "v1.0.0", s.cfg.MaxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	return m
}

func (s chunkScenario) instructions(t *testing.T) string {
	t.Helper()
	instructions, err := notes.ResolveInstructions(s.root, s.cfg.Release)
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}
	return instructions
}

// singlePass is the composed single-pass prompt for the scenario.
func (s chunkScenario) singlePass(t *testing.T) string {
	t.Helper()
	return notes.ComposePrompt(s.instructions(t), s.changeMap(t), s.diff())
}

// fixedPart is the single-pass prompt's size without its diff.
func (s chunkScenario) fixedPart(t *testing.T) int {
	t.Helper()
	return len(notes.ComposePrompt(s.instructions(t), s.changeMap(t), ""))
}

func (s chunkScenario) run(t *testing.T, mergeBody string) (*chunkTransport, string) {
	t.Helper()
	ct := &chunkTransport{mergeBody: mergeBody}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), ct, s.root)
	body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	return ct, body
}

func threeTenKBFiles() []chunkFile {
	return []chunkFile{{"alpha/a.go", 10_000}, {"beta/b.go", 10_000}, {"gamma/c.go", 10_000}}
}

func TestGenerator_UnderCap_DiffPast50000Lines_MakesOneSinglePassCall(t *testing.T) {
	t.Parallel()

	s := chunkScenario{
		files: []chunkFile{{"gen/big.go", 0}},
		cfg:   config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes},
		root:  t.TempDir(),
	}
	// 60,000 short lines: past the old line ceiling, well under the byte cap.
	diff := "diff --git a/gen/big.go b/gen/big.go\n@@ -0,0 +1,60000 @@\n" + strings.Repeat("+x\n", 60_000)
	r := seedNormalPathGit(t, diff, s.nameStatus(), s.numstat())
	ct := &chunkTransport{mergeBody: "single-pass notes"}

	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), ct, s.root)
	body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	want := notes.ComposePrompt(s.instructions(t), s.changeMap(t), diff)
	if len(want) > config.DefaultMaxPromptBytes {
		t.Fatalf("fixture prompt is %d bytes, want it under the default cap", len(want))
	}
	if len(ct.prompts) != 1 {
		t.Fatalf("AI calls = %d, want exactly one single-pass call", len(ct.prompts))
	}
	if ct.prompts[0] != want {
		t.Errorf("the one call is not the composed single-pass prompt")
	}
	if body != "single-pass notes" {
		t.Errorf("body = %q, want the single call's output", body)
	}
}

func TestGenerator_PromptExactlyAtCap_MakesOneSinglePassCall(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = len(s.singlePass(t))
	want := s.singlePass(t)

	ct, _ := s.run(t, "notes")

	if len(ct.prompts) != 1 || ct.prompts[0] != want {
		t.Errorf("AI calls = %d, want exactly one carrying the single-pass prompt at the cap", len(ct.prompts))
	}
}

func TestGenerator_DiffUnderCapButPromptOver_ChunksWithNoSinglePassCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "context.md"), []byte(strings.Repeat("Project convention text.\n", 500)), 0o644); err != nil {
		t.Fatal(err)
	}
	s := chunkScenario{files: []chunkFile{{"alpha/a.go", 20_000}, {"beta/b.go", 20_000}, {"gamma/c.go", 20_000}}, root: root}
	s.cfg = config.Config{MaxPromptBytes: 70_000}
	s.cfg.Release.Context = "context.md"
	single := s.singlePass(t)
	if len(s.diff()) >= s.cfg.MaxPromptBytes || len(single) <= s.cfg.MaxPromptBytes {
		t.Fatalf("fixture: diff %d bytes and single-pass %d bytes, want diff under and prompt over the %d cap", len(s.diff()), len(single), s.cfg.MaxPromptBytes)
	}

	ct, _ := s.run(t, "merged notes")

	for _, p := range ct.prompts {
		if p == single {
			t.Fatal("a single-pass call was made for an over-cap prompt")
		}
	}
	if got := len(ct.chunkPrompts()); got < 2 {
		t.Errorf("chunk calls = %d, want the notes chunked", got)
	}
	if got := len(ct.mergePrompts()); got != 1 {
		t.Errorf("merge calls = %d, want exactly one", got)
	}
}

func TestGenerator_OverCap_EachChunkCallCarriesItsPartsInOrder(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.Release.Context = "Our users are operators."
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000
	instructions := s.instructions(t)
	changeMap := s.changeMap(t)

	ct, _ := s.run(t, "merged notes")

	chunks := chunksInPartOrder(t, ct)
	if len(chunks) != 3 {
		t.Fatalf("chunk calls = %d, want 3 (one file fits each)", len(chunks))
	}
	for i, p := range chunks {
		k, n := partOf(t, p)
		if k != i+1 || n != 3 {
			t.Errorf("chunk %d names part %d of %d, want part %d of 3", i, k, n, i+1)
		}
		if len(p) > s.cfg.MaxPromptBytes {
			t.Errorf("part %d is %d bytes, over the %d cap", k, len(p), s.cfg.MaxPromptBytes)
		}
		head := instructions + "\n\n" + changeMap + "\n\n"
		if !strings.HasPrefix(p, head) {
			t.Errorf("part %d does not open with the release's instructions then the whole-release Change Map", k)
		}
		framing := strings.Index(p, partFraming.FindString(p))
		if framing < len(head) || framing > strings.Index(p, "diff --git ") {
			t.Errorf("part %d framing sits outside the gap between the Change Map and its slice", k)
		}
		if got, want := sliceOf(p), fileDiff(s.files[i].path, s.files[i].size); got != want {
			t.Errorf("part %d slice is not %s's diff", k, s.files[i].path)
		}
		if !strings.HasSuffix(p, "\n\n"+notes.OutputReminder) {
			t.Errorf("part %d does not end with the closing reminder", k)
		}
	}
}

func TestGenerator_OverCap_PromptOverrideReplacesContextInEveryCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const override = "OVERRIDE: write terse release notes as a bullet list."
	if err := os.WriteFile(filepath.Join(root, "notes-prompt.md"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}
	s := chunkScenario{files: threeTenKBFiles(), root: root}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.Release.Prompt = "notes-prompt.md"
	s.cfg.Release.Context = "CONTEXT-MARKER must not reach the AI"
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000

	ct, _ := s.run(t, "merged notes")

	if len(ct.chunkPrompts()) < 2 || len(ct.mergePrompts()) != 1 {
		t.Fatalf("chunk calls = %d, merge calls = %d; want a chunked run", len(ct.chunkPrompts()), len(ct.mergePrompts()))
	}
	for _, p := range ct.prompts {
		if !strings.HasPrefix(p, override+"\n\n") {
			t.Errorf("a call does not open with the override file's contents:\n%.200s", p)
		}
		if strings.Contains(p, "CONTEXT-MARKER") {
			t.Errorf("a call carries the [release].context text under a [release].prompt override")
		}
	}
}

func TestGenerator_OverCap_PacksFilesInPathOrderFillingEachChunk(t *testing.T) {
	t.Parallel()

	// git emitted the files out of path order; packing restores it.
	s := chunkScenario{
		files: []chunkFile{{"core/c.go", 10_000}, {"api/a.go", 10_000}, {"ext/e.go", 10_000}, {"bin/b.go", 10_000}, {"docs/d.go", 10_000}},
		root:  t.TempDir(),
	}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 25_000
	d := func(p string) string { return fileDiff(p, 10_000) }

	ct, _ := s.run(t, "merged notes")

	chunks := chunksInPartOrder(t, ct)
	want := []string{d("api/a.go") + d("bin/b.go"), d("core/c.go") + d("docs/d.go"), d("ext/e.go")}
	if len(chunks) != len(want) {
		t.Fatalf("chunk calls = %d, want %d", len(chunks), len(want))
	}
	for i, p := range chunks {
		if got := sliceOf(p); got != want[i] {
			t.Errorf("part %d carries files starting %q, want those starting %q", i+1, firstPath(got), firstPath(want[i]))
		}
		if i+1 < len(chunks) {
			next := d(firstPath(sliceOf(chunks[i+1])))
			if len(p)+len(next) <= s.cfg.MaxPromptBytes {
				t.Errorf("part %d stopped while the next file's diff still fit", i+1)
			}
		}
	}
}

func TestGenerator_OverCap_QuotedPathPacksInItsPathOrderPlace(t *testing.T) {
	t.Parallel()

	// git quotes a path with non-ASCII bytes; in git's real path order it sits
	// between docs/a.md and zeta/z.go.
	const quoted = `diff --git "a/docs/caf\303\251.md" "b/docs/caf\303\251.md"`
	body := strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 10_000/64)
	quotedDiff := quoted + "\n@@ -1 +1,9 @@\n" + body
	diff := fileDiff("alpha/a.go", 10_000) + fileDiff("docs/a.md", 10_000) + quotedDiff + fileDiff("zeta/z.go", 10_000)
	const nameStatus = "M\talpha/a.go\nM\tdocs/a.md\nM\tdocs/caf\303\251.md\nM\tzeta/z.go\n"
	const numstat = "156\t0\talpha/a.go\n156\t0\tdocs/a.md\n156\t0\tdocs/caf\303\251.md\n156\t0\tzeta/z.go\n"
	root := t.TempDir()

	changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus, numstat), "/repo", notes.ExcludeConfig{}).
		BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	cfg := config.Config{MaxPromptBytes: len(notes.ComposePrompt(notes.DefaultPrompt, changeMap, "")) + 15_000}
	ct := &chunkTransport{mergeBody: "merged notes"}
	gen := notes.NewGenerator(notes.NewAssembler(seedNormalPathGit(t, diff, nameStatus, numstat), "/repo", notes.ExcludeConfig{}), ct, root)
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	var headers []string
	for _, p := range chunksInPartOrder(t, ct) {
		header, _, _ := strings.Cut(sliceOf(p), "\n")
		headers = append(headers, header)
	}
	want := []string{"diff --git a/alpha/a.go b/alpha/a.go", "diff --git a/docs/a.md b/docs/a.md", quoted, "diff --git a/zeta/z.go b/zeta/z.go"}
	if strings.Join(headers, "\n") != strings.Join(want, "\n") {
		t.Errorf("parts in order open with:\n%s\nwant:\n%s", strings.Join(headers, "\n"), strings.Join(want, "\n"))
	}
}

// overCapSections runs an over-cap Generate whose cap fits one ~10 KB section per
// chunk, over sections given in git's emitted order, and returns the chunk slices in
// part order.
func overCapSections(t *testing.T, sections []string) []string {
	t.Helper()
	diff := strings.Join(sections, "")
	const nameStatus = "M\tx.go\n"
	const numstat = "1\t0\tx.go\n"
	changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus, numstat), "/repo", notes.ExcludeConfig{}).
		BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	cfg := config.Config{MaxPromptBytes: len(notes.ComposePrompt(notes.DefaultPrompt, changeMap, "")) + 15_000}
	ct := &chunkTransport{mergeBody: "merged notes"}
	gen := notes.NewGenerator(notes.NewAssembler(seedNormalPathGit(t, diff, nameStatus, numstat), "/repo", notes.ExcludeConfig{}), ct, t.TempDir())
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	var slices []string
	for _, p := range chunksInPartOrder(t, ct) {
		slices = append(slices, sliceOf(p))
	}
	return slices
}

// section renders a file section under the given header lines with ~10 KB of hunk.
func section(headerLines ...string) string {
	return strings.Join(headerLines, "\n") + "\n@@ -1 +1,9 @@\n" + strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 10_000/64)
}

func TestGenerator_OverCap_NoPrefixHeadersPackEverySectionOnce(t *testing.T) {
	t.Parallel()

	// diff.noprefix=true drops git's a/ and b/ prefixes from the header.
	sections := []string{
		section("diff --git a/b.go a/b.go"),
		section("diff --git core/c.go b/core/c.go"),
		section("diff --git x x"),
	}

	slices := overCapSections(t, sections)

	joined := strings.Join(slices, "")
	for _, sec := range sections {
		if strings.Count(joined, sec) != 1 {
			t.Errorf("section %q is not carried exactly once across the chunks", strings.SplitN(sec, "\n", 2)[0])
		}
	}
	if len(joined) != len(strings.Join(sections, "")) {
		t.Errorf("the chunks carry %d bytes of diff, want %d", len(joined), len(strings.Join(sections, "")))
	}
}

func TestGenerator_OverCap_RenameKeysOnItsDestinationPath(t *testing.T) {
	t.Parallel()

	// git emits a rename in its new path's order: aa.go before mm.go.
	rename := section("diff --git a/zz.go b/aa.go", "similarity index 90%", "rename from zz.go", "rename to aa.go", "--- a/zz.go", "+++ b/aa.go")
	plain := section("diff --git a/mm.go b/mm.go", "--- a/mm.go", "+++ b/mm.go")

	slices := overCapSections(t, []string{rename, plain})

	if len(slices) != 2 || slices[0] != rename || slices[1] != plain {
		t.Errorf("parts in order open with %q, want the rename to aa.go first then mm.go", headerLines(slices))
	}
}

func headerLines(slices []string) []string {
	var out []string
	for _, sl := range slices {
		line, _, _ := strings.Cut(sl, "\n")
		out = append(out, line)
	}
	return out
}

func TestGenerator_OverCap_ChunksCarryEveryFileExactlyOnce(t *testing.T) {
	t.Parallel()

	s := chunkScenario{
		files: []chunkFile{{"a/1.go", 7_000}, {"a/2.go", 3_000}, {"b/3.go", 12_000}, {"c/4.go", 5_000}, {"c/5.go", 9_000}, {"d/6.go", 1_000}},
		root:  t.TempDir(),
	}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 14_000

	ct, _ := s.run(t, "merged notes")

	var joined strings.Builder
	for _, p := range chunksInPartOrder(t, ct) {
		if len(p) > s.cfg.MaxPromptBytes {
			t.Errorf("a chunk call is %d bytes, over the %d cap", len(p), s.cfg.MaxPromptBytes)
		}
		joined.WriteString(sliceOf(p))
	}
	if joined.String() != s.diff() {
		t.Errorf("the chunks' slices joined in part order do not reproduce the diff exactly once")
	}
}

func TestGenerator_OverCap_OneMergeCallShipsItsOutputByteIdentical(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000
	instructions := s.instructions(t)
	changeMap := s.changeMap(t)
	const mergeBody = "\n✨ Added\n- Three areas landed together   \n\n"

	ct, body := s.run(t, mergeBody)

	merges := ct.mergePrompts()
	if len(merges) != 1 {
		t.Fatalf("merge calls = %d, want exactly one", len(merges))
	}
	merge := merges[0]
	if ct.prompts[len(ct.prompts)-1] != merge {
		t.Errorf("the merge call was not the last call")
	}
	if !strings.HasPrefix(merge, instructions+"\n\n"+changeMap+"\n\n") {
		t.Errorf("merge does not open with the release's instructions then the same Change Map")
	}
	if !strings.Contains(merge, "one set of release notes") {
		t.Errorf("merge framing does not ask for one set of release notes:\n%s", merge)
	}
	last := -1
	for _, f := range s.files {
		at := strings.Index(merge, chunkNotesFor(f.path))
		if at < 0 {
			t.Fatalf("merge is missing the notes of the part opening with %s", f.path)
		}
		if at < last {
			t.Errorf("merge carries the part notes out of chunk order")
		}
		last = at
	}
	if strings.Contains(merge, "diff --git ") || strings.Contains(merge, "+xxxxxxxx") {
		t.Errorf("merge carries diff content")
	}
	if !strings.HasSuffix(merge, "\n\n"+notes.OutputReminder) {
		t.Errorf("merge does not end with the closing reminder")
	}
	if body != mergeBody {
		t.Errorf("body = %q, want the merge output byte-identical %q", body, mergeBody)
	}
}

func TestGenerator_GenerateFromRange_OverCap_ChunksOverThatRange(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000
	ct := &chunkTransport{mergeBody: "merged range notes"}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())

	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), ct, s.root)
	body, err := gen.GenerateFromRange(t.Context(), "v1.3.0..v1.4.0", s.cfg)
	if err != nil {
		t.Fatalf("GenerateFromRange returned unexpected error: %v", err)
	}

	if body != "merged range notes" {
		t.Errorf("body = %q, want the merge output", body)
	}
	if len(ct.chunkPrompts()) != 3 || len(ct.mergePrompts()) != 1 {
		t.Errorf("chunk calls = %d, merge calls = %d; want 3 and 1", len(ct.chunkPrompts()), len(ct.mergePrompts()))
	}
	invs := r.Invocations()
	if len(invs) != 3 {
		t.Fatalf("git invocations = %d, want 3", len(invs))
	}
	assertArgvEqual(t, invs[0].Args, wantRangeArgs("v1.3.0..v1.4.0"))
	assertArgvEqual(t, invs[1].Args, []string{"diff", "--name-status", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml"})
}

var _ notes.Transport = (*chunkTransport)(nil)
