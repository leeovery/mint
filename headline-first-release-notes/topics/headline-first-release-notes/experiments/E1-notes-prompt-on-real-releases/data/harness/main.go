// Command e1harness builds one release's notes input the way mint's regenerate
// fresh path builds it (the prev..tag diff and Change Map, with the release's own
// at-tag exclusions), composes it with one arm's instructions and closing reminder,
// and sends it through mint's own AI transport with the shipped default command.
//
// It is an experiment instrument, not product code: it lives in the experiment
// record and is built into the mint module through `go build -overlay` (see
// build.sh), so it imports mint's internal packages without touching the tree.
//
// The two arms differ ONLY in the instructions and the reminder. The current arm
// composes with notes.ComposePrompt(notes.DefaultPrompt, …), byte-for-byte what
// mint ships; the new arm joins the same four parts in the same order with the
// candidate texts read from files.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// tagConfig is a LENIENT read of the release's .mint.toml as it stood at the tag:
// only the keys that shape the notes diff, at either level, unknown keys ignored
// (old schemas carry keys today's strict decoder rejects).
type tagConfig struct {
	DiffExclude    []string `toml:"diff_exclude"`
	VersionFile    string   `toml:"version_file"`
	VersionPattern string   `toml:"version_pattern"`
	Release        struct {
		DiffExclude    []string `toml:"diff_exclude"`
		VersionFile    string   `toml:"version_file"`
		VersionPattern string   `toml:"version_pattern"`
	} `toml:"release"`
}

type meta struct {
	Repo               string   `json:"repo"`
	Prev               string   `json:"prev"`
	Tag                string   `json:"tag"`
	Arm                string   `json:"arm"`
	InstructionsSHA256 string   `json:"instructions_sha256"`
	ReminderSHA256     string   `json:"reminder_sha256"`
	ExcludeGlobs       []string `json:"exclude_globs"`
	VersionFile        string   `json:"version_file"`
	VersionPatternSet  bool     `json:"version_pattern_set"`
	PromptBytes        int      `json:"prompt_bytes"`
	PromptSHA256       string   `json:"prompt_sha256"`
	InputSHA256        string   `json:"input_sha256"`
	AICommand          string   `json:"ai_command"`
	TimeoutSeconds     float64  `json:"timeout_seconds"`
	Started            string   `json:"started"`
	Finished           string   `json:"finished"`
	Seconds            float64  `json:"seconds"`
	BodyBytes          int      `json:"body_bytes"`
	Error              string   `json:"error,omitempty"`
}

func main() {
	repo := flag.String("repo", "", "release repo root")
	prev := flag.String("prev", "", "previous tag")
	tag := flag.String("tag", "", "release tag")
	arm := flag.String("arm", "", "current | new")
	instructionsFile := flag.String("instructions", "", "new arm: instructions file")
	reminderFile := flag.String("reminder", "", "new arm: closing reminder file")
	promptOut := flag.String("prompt-out", "", "write the composed prompt here")
	inputOut := flag.String("input-out", "", "write the Change Map + diff (what the AI was given besides instructions) here")
	bodyOut := flag.String("body-out", "", "write the notes body here")
	metaOut := flag.String("meta-out", "", "write the run metadata (JSON) here")
	timeout := flag.Duration("timeout", 600*time.Second, "per-attempt AI deadline")
	assembleOnly := flag.Bool("assemble-only", false, "build and write the prompt, make no AI call")
	flag.Parse()

	m := meta{Repo: *repo, Prev: *prev, Tag: *tag, Arm: *arm, AICommand: config.DefaultAICommand, TimeoutSeconds: timeout.Seconds()}
	err := run(&m, *instructionsFile, *reminderFile, *promptOut, *inputOut, *bodyOut, *timeout, *assembleOnly)
	if err != nil {
		m.Error = err.Error()
	}
	if *metaOut != "" {
		data, _ := json.MarshalIndent(m, "", "  ")
		if werr := os.WriteFile(*metaOut, append(data, '\n'), 0o644); werr != nil {
			fmt.Fprintln(os.Stderr, "writing meta:", werr)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(m *meta, instructionsFile, reminderFile, promptOut, inputOut, bodyOut string, timeout time.Duration, assembleOnly bool) error {
	ctx := context.Background()
	r := runner.NewExecRunner()

	instructions, reminder, err := armTexts(m.Arm, instructionsFile, reminderFile)
	if err != nil {
		return err
	}
	m.InstructionsSHA256, m.ReminderSHA256 = digest(instructions), digest(reminder)

	cfg, err := readTagConfig(ctx, r, m.Repo, m.Tag)
	if err != nil {
		return err
	}
	exclude := notes.ExcludeConfig{
		Globs:          append(append([]string{}, cfg.DiffExclude...), cfg.Release.DiffExclude...),
		VersionFile:    firstNonEmpty(cfg.Release.VersionFile, cfg.VersionFile),
		VersionPattern: firstNonEmpty(cfg.Release.VersionPattern, cfg.VersionPattern),
	}
	m.ExcludeGlobs, m.VersionFile, m.VersionPatternSet = exclude.Globs, exclude.VersionFile, exclude.VersionPattern != ""

	asm := notes.NewAssembler(r, m.Repo, exclude)
	diffRange := m.Prev + ".." + m.Tag
	diff, err := asm.AssembleRange(ctx, diffRange)
	if err != nil {
		return err
	}
	if notes.IsDegenerate(diff) {
		return errors.New("empty post-exclusion diff")
	}
	changeMap, err := asm.BuildChangeMapForRange(ctx, diffRange, config.DefaultMaxPromptBytes)
	if err != nil {
		return err
	}

	var prompt string
	if m.Arm == "current" {
		prompt = notes.ComposePrompt(notes.DefaultPrompt, changeMap, diff)
	} else {
		prompt = strings.Join([]string{instructions, changeMap, diff, reminder}, "\n\n")
	}
	input := changeMap + "\n\n" + diff
	m.PromptBytes, m.PromptSHA256, m.InputSHA256 = len(prompt), digest(prompt), digest(input)
	if err := writeIf(promptOut, prompt); err != nil {
		return err
	}
	if err := writeIf(inputOut, input); err != nil {
		return err
	}
	if assembleOnly {
		return nil
	}

	transport := ai.NewTransport(r, ai.Config{AICommand: config.DefaultAICommand, Timeout: &timeout})
	start := time.Now()
	m.Started = start.UTC().Format(time.RFC3339)
	body, genErr := transport.Generate(ctx, prompt)
	end := time.Now()
	m.Finished, m.Seconds, m.BodyBytes = end.UTC().Format(time.RFC3339), end.Sub(start).Seconds(), len(body)
	if genErr != nil {
		return fmt.Errorf("generating notes: %w", genErr)
	}
	return writeIf(bodyOut, body)
}

// armTexts returns the arm's instructions and closing reminder: mint's shipped
// constants for the current arm, the candidate files for the new arm.
func armTexts(arm, instructionsFile, reminderFile string) (string, string, error) {
	switch arm {
	case "current":
		return notes.DefaultPrompt, notes.OutputReminder, nil
	case "new":
		instructions, err := os.ReadFile(instructionsFile)
		if err != nil {
			return "", "", err
		}
		reminder, err := os.ReadFile(reminderFile)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(instructions), "\n"), strings.TrimRight(string(reminder), "\n"), nil
	default:
		return "", "", fmt.Errorf("unknown arm %q", arm)
	}
}

func readTagConfig(ctx context.Context, r runner.CommandRunner, repo, tag string) (tagConfig, error) {
	var cfg tagConfig
	res, err := r.RunInDir(ctx, repo, nil, "git", "show", tag+":.mint.toml")
	if err != nil {
		return cfg, nil // no .mint.toml at the tag: built-in excludes only
	}
	if err := toml.Unmarshal([]byte(res.Stdout), &cfg); err != nil {
		return cfg, fmt.Errorf("reading %s:.mint.toml: %w", tag, err)
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeIf(path, content string) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
