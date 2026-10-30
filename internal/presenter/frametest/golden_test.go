package frametest_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/presenter/frametest"
)

func TestAFrameWrittenIsKeptAsItsANSIAndItsTextAndHeldToWhatDrewIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drawn := map[string]string{"done-100": " \x1b[36m●\x1b[39m  \x1b[1mpush\x1b[22m  ✨\n\n next\n"}

	frametest.Write(t, dir, drawn)

	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(dir, "done-100.ansi"), filepath.Join(dir, "done-100.txt")}; !slices.Equal(files, want) {
		t.Errorf("the frames written are %v, want %v, the stale one deleted", files, want)
	}
	want := map[string]string{
		"done-100.ansi": drawn["done-100"],
		"done-100.txt":  " ●  push  ✨\n\n next\n\n",
	}
	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != content {
			t.Errorf("%s holds %q, want %q", name, got, content)
		}
	}
	frametest.Check(t, dir, drawn)
}
