package runner

import "context"

type stopNoticeKey struct{}

// WithStopNotice returns ctx carrying notice, which a graceful runner calls once it has
// sent SIGTERM to a command still running when ctx ended, before it waits for the
// command to stop: a command that never started, or had exited, gets none. The runner
// calls it on a goroutine of its own and returns from the run only after it has, so
// what notice draws comes before whatever follows the run. It rides the context, as
// net/http/httptrace's hooks do, because the CommandRunner seam carries nothing else
// per call.
func WithStopNotice(ctx context.Context, notice func()) context.Context {
	return context.WithValue(ctx, stopNoticeKey{}, notice)
}

// NoticeStop calls the notice ctx carries, where it carries one: what a runner does once
// it has asked a command run under ctx to stop.
func NoticeStop(ctx context.Context) {
	if notice, ok := ctx.Value(stopNoticeKey{}).(func()); ok {
		notice()
	}
}
