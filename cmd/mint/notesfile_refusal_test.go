package main

import (
	"context"
	"testing"
)

func TestParseReleaseFlags_NotesFileWithNoAI_IsAUsageError(t *testing.T) {
	t.Parallel()

	const wantMsg = "can't combine --notes-file with --no-ai"
	for _, args := range [][]string{
		{"--notes-file", "notes.md", "--no-ai"},
		{"--no-ai", "--notes-file", "notes.md"},
		{"--no-ai", "--notes-file", ""},
	} {
		_, err := parseReleaseFlags(args)
		if err == nil {
			t.Fatalf("parseReleaseFlags(%q) returned nil error, want %q", args, wantMsg)
		}
		if err.Error() != wantMsg {
			t.Errorf("parseReleaseFlags(%q) error = %q, want %q", args, err.Error(), wantMsg)
		}
	}
}

func TestRunRelease_NotesFileWithNoAI_ExitsWithTheUsageCode(t *testing.T) {
	t.Parallel()

	if code := runRelease(context.Background(), []string{"--notes-file", "notes.md", "--no-ai"}); code != usageExitCode {
		t.Errorf("mint release --notes-file notes.md --no-ai exited %d, want the usage error %d", code, usageExitCode)
	}
}

func TestParseReleaseFlags_EmptyNotesFilePath_CountsAsTheFlagGiven(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args    []string
		wantSet bool
	}{
		{args: []string{"--notes-file", "", "-y"}, wantSet: true},
		{args: []string{"--notes-file=", "-y"}, wantSet: true},
		{args: []string{"--notes-file", "notes.md"}, wantSet: true},
		{args: []string{"-y"}, wantSet: false},
	} {
		flags, err := parseReleaseFlags(tc.args)
		if err != nil {
			t.Fatalf("parseReleaseFlags(%q) returned error: %v", tc.args, err)
		}
		if flags.NotesFileSet != tc.wantSet {
			t.Errorf("parseReleaseFlags(%q) NotesFileSet = %v, want %v", tc.args, flags.NotesFileSet, tc.wantSet)
		}
		if got := flags.ReleaseOptions().NotesFileSet; got != tc.wantSet {
			t.Errorf("parseReleaseFlags(%q).ReleaseOptions().NotesFileSet = %v, want %v", tc.args, got, tc.wantSet)
		}
	}
}

func TestParseRegenerateFlags_NotesFileWithAllOrSource_IsAUsageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{name: "--all", args: []string{"--all", "--notes-file", "notes.md"}, wantMsg: "cannot combine --notes-file with --all"},
		{name: "--all with an empty path", args: []string{"--notes-file", "", "--all"}, wantMsg: "cannot combine --notes-file with --all"},
		{name: "--source fresh", args: []string{"1.4.0", "--notes-file", "notes.md", "--source", "fresh"}, wantMsg: "cannot combine --notes-file with --source"},
		{name: "--source tag", args: []string{"1.4.0", "--notes-file", "notes.md", "--source", "tag"}, wantMsg: "cannot combine --notes-file with --source"},
		{name: "--source release", args: []string{"--source=release", "1.4.0", "--notes-file", "notes.md"}, wantMsg: "cannot combine --notes-file with --source"},
		{name: "--source with an empty value", args: []string{"1.4.0", "--notes-file", "notes.md", "--source", ""}, wantMsg: "cannot combine --notes-file with --source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseRegenerateFlags(tt.args)
			if err == nil {
				t.Fatalf("parseRegenerateFlags(%q) returned nil error, want %q", tt.args, tt.wantMsg)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("parseRegenerateFlags(%q) error = %q, want %q", tt.args, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestRunRegenerate_NotesFileWithAllOrSource_ExitsWithTheUsageCode(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"--all", "--notes-file", "notes.md"},
		{"1.4.0", "--notes-file", "notes.md", "--source", "fresh"},
	} {
		if code := runRegenerate(context.Background(), args); code != usageExitCode {
			t.Errorf("mint release regenerate %q exited %d, want the usage error %d", args, code, usageExitCode)
		}
	}
}

func TestRegenerateRunFlags_EmptyNotesFilePath_CountsAsTheFlagGiven(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args    []string
		wantSet bool
	}{
		{args: []string{"1.4.0", "--notes-file", "", "--target", "both", "-y"}, wantSet: true},
		{args: []string{"1.4.0", "--notes-file=", "--target", "both", "-y"}, wantSet: true},
		{args: []string{"1.4.0", "--target", "both", "-y"}, wantSet: false},
	} {
		req, err := parseRegenerateFlags(tc.args)
		if err != nil {
			t.Fatalf("parseRegenerateFlags(%q) returned error: %v", tc.args, err)
		}
		if got := regenerateRunFlags(req).NotesFileSet; got != tc.wantSet {
			t.Errorf("regenerateRunFlags(%q).NotesFileSet = %v, want %v", tc.args, got, tc.wantSet)
		}
	}
}
