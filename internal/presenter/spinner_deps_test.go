package presenter_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSpinnerLibraryIsNotBubbleTea is the dependency guard for the spinner library
// choice: the spec bans Bubble Tea (no alt-screen, no event-loop ownership). It
// runs `go list -deps` on the presenter package and asserts no
// charmbracelet/bubbletea is reachable in the dependency tree — proving the chosen
// standalone spinner (briandowns) drags in no full-screen TUI runtime.
func TestSpinnerLibraryIsNotBubbleTea(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "mint/internal/presenter")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps failed: %v", err)
	}

	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(dep, "charmbracelet/bubbletea") {
			t.Errorf("banned dependency reachable from presenter: %q — Bubble Tea is forbidden", dep)
		}
	}
}
