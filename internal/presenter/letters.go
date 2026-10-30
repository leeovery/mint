package presenter

import (
	"strings"
	"unicode/utf8"

	"mint/internal/presenter/rail"
)

// lettering is how the rail reads an answer to a gate whose keys are words, such as
// regenerate's Source and Target: each word answers to its first letter, a single
// keypress, as every other question's keys do. Only the rail letters a gate; plain
// reads the words as typed.
type lettering struct {
	// gate is the gate asked, keyed by its words.
	gate Gate
	// keyed is gate with each choice keyed by its letter and the default by the
	// default's, or gate itself where it isn't lettered.
	keyed Gate
	// letters holds the letter each of gate's choices answers to, in its order; nil
	// where gate isn't lettered.
	letters []Choice
}

// letter is g as the rail reads it: lettered where every key is a word and no two
// words start alike, case aside. A gate whose keys are one character already needs
// no letters, and one with two words alike keeps a typed word rather than guess
// which the letter means.
func letter(g Gate) lettering {
	l := lettering{gate: g, keyed: g}
	keys := firstLetters(g)
	if keys == nil {
		return l
	}
	l.letters = keys
	l.keyed.Choices = make([]GateChoice, len(g.Choices))
	for i, c := range g.Choices {
		l.keyed.Choices[i] = c
		l.keyed.Choices[i].Key = keys[i]
		if c.Key == g.Default {
			l.keyed.Default = keys[i]
		}
	}
	return l
}

// firstLetters is each of g's keys' first letter, lower-cased, where g's keys are
// words whose first letters all differ; nil where not.
func firstLetters(g Gate) []Choice {
	if singleKeyGate(g) {
		return nil
	}
	keys := make([]Choice, 0, len(g.Choices))
	seen := make(map[Choice]bool, len(g.Choices))
	for _, c := range g.Choices {
		r, _ := utf8.DecodeRuneInString(string(c.Key))
		k := Choice(strings.ToLower(string(r)))
		if seen[k] {
			return nil
		}
		seen[k] = true
		keys = append(keys, k)
	}
	return keys
}

// word is the gate's choice the keyed choice c stands for: the word a letter stands
// for, or c itself where it is no letter of the gate's.
func (l lettering) word(c Choice) Choice {
	for i, k := range l.letters {
		if k == c {
			return l.gate.Choices[i].Key
		}
	}
	return c
}

// parse is the choice a typed line names: a letter, or, as plain reads it, a word.
func (l lettering) parse(line string) (Choice, bool) {
	if c, ok := parseChoice(line, l.keyed); ok {
		return l.word(c), true
	}
	return parseChoice(line, l.gate)
}

// choices is the gate's keys line: each choice's key, its action and whether it is
// the default. A lettered choice draws its letter as the key and its word as the
// action, with what the word means as what it is about, unless that only repeats it.
func (l lettering) choices() []rail.Choice {
	out := make([]rail.Choice, 0, len(l.gate.Choices))
	for i, c := range l.gate.Choices {
		rc := rail.Choice{Key: string(c.Key), Action: c.Action, Default: c.Key == l.gate.Default}
		if l.letters != nil {
			rc.Key, rc.Action = string(l.letters[i]), string(c.Key)
			if !strings.EqualFold(c.Action, string(c.Key)) {
				rc.About = c.Action
			}
		}
		out = append(out, rc)
	}
	return out
}
