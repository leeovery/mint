package notes

import (
	"regexp"
	"strings"
)

// pathspecMatches reports whether git matches path to spec, a pathspec without magic:
// spec names path itself or a leading directory of it, or, holding a wildcard, matches
// the whole path with * and ? crossing "/" (git's wildmatch without WM_PATHNAME). A
// malformed bracket expression matches only literally, as git's does.
func pathspecMatches(spec, path string) bool {
	if rest, ok := strings.CutPrefix(path, spec); ok {
		if rest == "" || strings.HasSuffix(spec, "/") || strings.HasPrefix(rest, "/") {
			return true
		}
	}
	if !strings.ContainsAny(spec, `*?[\`) {
		return false
	}
	pattern, ok := wildmatchPattern(spec)
	return ok && pattern.MatchString(path)
}

// characterClasses maps the bracket-expression class names wildmatch accepts to their
// regexp form.
var characterClasses = map[string]string{
	"alnum": `[:alnum:]`, "alpha": `[:alpha:]`, "blank": `[:blank:]`, "cntrl": `[:cntrl:]`,
	"digit": `[:digit:]`, "graph": `[:graph:]`, "lower": `[:lower:]`, "print": `[:print:]`,
	"punct": `[:punct:]`, "space": `[:space:]`, "upper": `[:upper:]`, "xdigit": `[:xdigit:]`,
}

// wildmatchPattern translates spec into an anchored regexp, false when wildmatch would
// abort on it.
func wildmatchPattern(spec string) (*regexp.Regexp, bool) {
	var re strings.Builder
	re.WriteString(`(?s)\A`)
	for i := 0; i < len(spec); i++ {
		switch c := spec[i]; c {
		case '*':
			re.WriteString(`.*`)
		case '?':
			re.WriteString(`.`)
		case '\\':
			i++
			if i == len(spec) {
				return nil, false
			}
			re.WriteString(regexp.QuoteMeta(spec[i : i+1]))
		case '[':
			class, end, ok := bracketExpression(spec, i)
			if !ok {
				return nil, false
			}
			re.WriteString(class)
			i = end
		default:
			re.WriteString(regexp.QuoteMeta(spec[i : i+1]))
		}
	}
	re.WriteString(`\z`)
	pattern, err := regexp.Compile(re.String())
	return pattern, err == nil
}

// bracketExpression translates the bracket expression opening at spec[open] into a
// regexp class, returning the index of its closing "]". As in wildmatch, a "]" first in
// the set is a member, "!" or "^" first negates, and a "-" between two members is a range.
func bracketExpression(spec string, open int) (string, int, bool) {
	var class strings.Builder
	class.WriteString("[")
	i := open + 1
	if i < len(spec) && (spec[i] == '!' || spec[i] == '^') {
		class.WriteString("^")
		i++
	}
	for first := true; i < len(spec); first = false {
		c := spec[i]
		switch {
		case c == ']' && !first:
			class.WriteString("]")
			return class.String(), i, true
		case c == '\\' && i+1 < len(spec):
			class.WriteString(regexp.QuoteMeta(spec[i+1 : i+2]))
			i += 2
		case c == '[' && strings.HasPrefix(spec[i:], "[:"):
			name, _, found := strings.Cut(spec[i+2:], ":]")
			named, known := characterClasses[name]
			if !found || !known {
				return "", 0, false
			}
			class.WriteString(named)
			i += len("[:") + len(name) + len(":]")
		case c == '-' && !first && i+1 < len(spec) && spec[i+1] != ']':
			class.WriteString("-")
			i++
		default:
			class.WriteString(regexp.QuoteMeta(spec[i : i+1]))
			i++
		}
	}
	return "", 0, false
}
