package frametest

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write the golden frames again from what the tests draw")

// The kinds of file a frame is kept as: its colours and its text.
const (
	ansiExt = ".ansi"
	textExt = ".txt"
)

// Check holds each frame drawn, by its name, to the one dir keeps by that name,
// cell for cell, and fails on a frame dir keeps that nothing draws. Run with
// -update, it writes the frames drawn there instead, and deletes the rest.
func Check(t *testing.T, dir string, drawn map[string]string) {
	t.Helper()
	if *update {
		write(t, dir, drawn)
		return
	}
	for _, name := range kept(t, dir) {
		if _, ok := drawn[name]; !ok {
			t.Errorf("nothing draws the frame %s: draw it, or delete it", name)
		}
	}
	for name, frame := range drawn {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			held(t, dir, name, frame)
		})
	}
}

// held holds frame, drawn as name, to the frame dir keeps by that name: its ANSI
// read as its text says, and every cell as drawn.
func held(t *testing.T, dir, name, frame string) {
	t.Helper()
	want := read(t, dir, name)
	if text := lines(t, dir, name); !slices.Equal(want.Text(), text) {
		t.Fatalf("%s%s reads\n%s\nwant its %s\n%s", name, ansiExt, strings.Join(want.Text(), "\n"), textExt, strings.Join(text, "\n"))
	}
	got, err := Parse(frame)
	if err != nil {
		t.Fatalf("parsing what draws %s: %v", name, err)
	}
	diffs, err := Compare(want, got)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(diffs) > 0 {
		t.Error(Describe(name, diffs))
	}
}

func read(t *testing.T, dir, name string) Grid {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+ansiExt))
	if err != nil {
		t.Fatalf("%v: write the frames with -update", err)
	}
	g, err := Parse(string(data))
	if err != nil {
		t.Fatalf("parsing %s%s: %v", name, ansiExt, err)
	}
	return g
}

func lines(t *testing.T, dir, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+textExt))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// kept are the names of the frames dir keeps, by either of their files.
func kept(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if name := strings.TrimSuffix(e.Name(), ext); (ext == ansiExt || ext == textExt) && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// write keeps each frame drawn in dir as its ANSI, with no newline after its last
// row, and its text, and deletes the frames nothing draws.
func write(t *testing.T, dir string, drawn map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range kept(t, dir) {
		for _, ext := range []string{ansiExt, textExt} {
			if err := os.Remove(filepath.Join(dir, name+ext)); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
	for name, frame := range drawn {
		g, err := Parse(frame)
		if err != nil {
			t.Fatalf("parsing what draws %s: %v", name, err)
		}
		text := strings.Join(g.Text(), "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, name+ansiExt), []byte(strings.TrimSuffix(frame, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+textExt), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
