// Package cmdline splits a command line into a program name and its arguments by POSIX
// shell quoting rules, without being a shell: quotes, backslashes and line continuations
// are honoured, and nothing else is syntax — `$`, backticks, `*`, `~`, `|`, `;`, `&`, `<`
// and `>` are ordinary word characters. Callers run the resulting words directly.
package cmdline

import (
	"errors"
	"strings"
	"unicode"
)

var (
	// ErrUnterminatedQuote is a single or double quote with no closing partner.
	ErrUnterminatedQuote = errors.New("unterminated quote")

	// ErrTrailingBackslash is a backslash outside quotes with nothing after it.
	ErrTrailingBackslash = errors.New("trailing lone backslash")
)

// Split returns the words of command, or nil and ErrUnterminatedQuote /
// ErrTrailingBackslash when it cannot be split. A blank command yields no words.
// Unquoted whitespace is any unicode.IsSpace rune, so a valid UTF-8 command with no
// quotes or backslashes splits exactly as strings.Fields does.
func Split(command string) ([]string, error) {
	s := splitter{input: []rune(command)}
	for s.pos < len(s.input) {
		if err := s.step(); err != nil {
			return nil, err
		}
	}
	s.endWord()
	return s.words, nil
}

type splitter struct {
	input []rune
	pos   int
	words []string
	word  strings.Builder
	// inWord is tracked apart from word.Len() so a quoted empty string is still a word.
	inWord bool
}

func (s *splitter) step() error {
	r := s.input[s.pos]
	s.pos++
	switch {
	case r == '\'':
		return s.singleQuoted()
	case r == '"':
		return s.doubleQuoted()
	case r == '\\':
		return s.escaped()
	case unicode.IsSpace(r):
		s.endWord()
	default:
		s.appendRune(r)
	}
	return nil
}

func (s *splitter) singleQuoted() error {
	s.inWord = true
	for s.pos < len(s.input) {
		r := s.input[s.pos]
		s.pos++
		if r == '\'' {
			return nil
		}
		s.word.WriteRune(r)
	}
	return ErrUnterminatedQuote
}

func (s *splitter) doubleQuoted() error {
	s.inWord = true
	for s.pos < len(s.input) {
		r := s.input[s.pos]
		s.pos++
		switch {
		case r == '"':
			return nil
		case r == '\\' && s.pos < len(s.input) && strings.ContainsRune("\"\\$`\n", s.input[s.pos]):
			if next := s.input[s.pos]; next != '\n' {
				s.word.WriteRune(next)
			}
			s.pos++
		default:
			s.word.WriteRune(r)
		}
	}
	return ErrUnterminatedQuote
}

func (s *splitter) escaped() error {
	if s.pos == len(s.input) {
		return ErrTrailingBackslash
	}
	r := s.input[s.pos]
	s.pos++
	if r != '\n' {
		s.appendRune(r)
	}
	return nil
}

func (s *splitter) appendRune(r rune) {
	s.inWord = true
	s.word.WriteRune(r)
}

func (s *splitter) endWord() {
	if !s.inWord {
		return
	}
	s.words = append(s.words, s.word.String())
	s.word.Reset()
	s.inWord = false
}
