package runner

import "context"

type (
	stopNoticeKey struct{}
	killNoticeKey struct{}
)

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

// WithKillNotice returns ctx carrying notice, which a graceful runner calls once
// KillStopping has killed a command run under ctx, the second press forcing it: a
// command that stopped on its SIGTERM, or had exited, gets none. The runner calls it
// once, on the goroutine running the command, after the command's output is read and
// before the run returns, never on KillStopping's, so what notice draws can't land
// after whatever follows the run.
func WithKillNotice(ctx context.Context, notice func()) context.Context {
	return context.WithValue(ctx, killNoticeKey{}, notice)
}

// NoticeStop calls the stop notice ctx carries, where it carries one: what a runner
// does once it has asked a command run under ctx to stop.
func NoticeStop(ctx context.Context) { notify(ctx, stopNoticeKey{}) }

// NoticeKill calls the kill notice ctx carries, where it carries one: what a runner
// does once a command run under ctx was killed while it stopped.
func NoticeKill(ctx context.Context) { notify(ctx, killNoticeKey{}) }

func notify(ctx context.Context, key any) {
	if notice, ok := ctx.Value(key).(func()); ok {
		notice()
	}
}
