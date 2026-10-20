package git

// Exclusion is one path, relative to the repo root and slash-separated, that a git
// step leaves out of what it inspects, stashes, stages or commits. The zero value
// excludes nothing.
type Exclusion string

// Pathspec returns the trailing `-- <pathspec>` arguments that leave the path out,
// or nil for the zero value so the command runs exactly as it would without one.
// The pathspec is root-anchored and literal, so it means the same file from any
// working directory and glob characters in the name match only themselves. With no
// other pathspec beside it, the command covers what it would with no pathspec at all,
// less the path.
func (e Exclusion) Pathspec() []string {
	if e == "" {
		return nil
	}
	return []string{"--", ":(top,literal,exclude)" + string(e)}
}

// LiteralPathspec is the pathspec matching exactly path, relative to the repo root
// and slash-separated, from any working directory.
func LiteralPathspec(path string) string {
	return ":(top,literal)" + path
}
