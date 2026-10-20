package cmdline

import "strings"

// Quote is Split's inverse for one word: it returns word written so that Split, or a
// POSIX shell, reads it back as that one word whole. A word made only of the bytes
// [A-Za-z0-9_@%+=:,./-] comes back unchanged. Any other word is wrapped in single
// quotes, each single quote inside it closed, backslash-escaped and reopened; an empty
// word is a pair of single quotes.
func Quote(word string) string {
	if word == "" {
		return "''"
	}
	if strings.IndexFunc(word, needsQuoting) == -1 {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func needsQuoting(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}
