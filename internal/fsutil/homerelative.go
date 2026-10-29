package fsutil

import "path/filepath"

// HomeRelative is path with a leading home directory shortened to "~": home itself
// reads "~" and a path below it "~/…". A path outside home, or an empty home, is
// returned as it is.
func HomeRelative(path, home string) string {
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	switch {
	case err != nil:
		return path
	case rel == ".":
		return "~"
	case filepath.IsLocal(rel):
		return "~" + string(filepath.Separator) + rel
	default:
		return path
	}
}
