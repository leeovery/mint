//go:build unix

package commit_test

// The untracked counts that take a FIFO, which only a Unix filesystem makes.

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRun_AnUntrackedLinkToAFIFOIsNeverOpened(t *testing.T) {
	t.Parallel()

	done := make(chan string, 1)
	go func() {
		row, _ := untrackedRow(t, "pipe\x00", func(root string) {
			fifo := filepath.Join(root, "fifo")
			if err := syscall.Mkfifo(fifo, 0o644); err != nil {
				t.Error(err)
			}
			link(t, "fifo", filepath.Join(root, "pipe"))
		})
		done <- row
	}()
	select {
	case row := <-done:
		if want := "1 file · +1 −0 · everything, staged when you accept"; row != want {
			t.Errorf("diff row = %q, want %q: the link, not what it points at", row, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the preflight blocked on a link to a FIFO, which it must never open")
	}
}

// TestRun_AMixedTreeCountsAsGitsNumstatForTheCommit holds the count of every kind of
// untracked entry together to what `git diff --cached --numstat` reads once `git add
// -A` has staged them: text 3, unterminated text 2, empty 0, binary -, a nested repo 1,
// a link 1, and a FIFO, which git won't add, left out.
func TestRun_AMixedTreeCountsAsGitsNumstatForTheCommit(t *testing.T) {
	t.Parallel()

	listing := "a.txt\x00b.txt\x00empty\x00logo.png\x00sub/\x00link\x00"
	row, _ := untrackedRow(t, listing+"pipe\x00", func(root string) {
		putFile(t, filepath.Join(root, "a.txt"), []byte("1\n2\n3\n"))
		putFile(t, filepath.Join(root, "b.txt"), []byte("1\n2"))
		putFile(t, filepath.Join(root, "empty"), nil)
		putFile(t, filepath.Join(root, "logo.png"), []byte{0x89, 'P', 'N', 'G', 0, 0, '\n'})
		putFile(t, filepath.Join(root, "sub", ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
		link(t, "a.txt", filepath.Join(root, "link"))
		if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if want := "6 files · +7 −0 · everything, staged when you accept"; row != want {
		t.Errorf("diff row = %q, want %q", row, want)
	}
}
