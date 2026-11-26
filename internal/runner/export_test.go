package runner

// StopsGracefully reports whether r gives a command whose context ends a Cancel of
// its own, where one that doesn't leaves exec's, which kills it at once.
func StopsGracefully(r *ExecRunner) bool { return r.graceful != nil }
