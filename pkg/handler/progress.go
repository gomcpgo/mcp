package handler

import "context"

// ProgressReporter is the handler-facing API for emitting progress updates
// on a long-running request. Handlers obtain a reporter via
// ProgressReporterFromContext(ctx) and call Report unconditionally — if the
// client did not include a progressToken on the request, the reporter is a
// no-op and the call is silently dropped.
type ProgressReporter interface {
	// Report emits a single progress update. total may be nil for
	// indeterminate progress. message is optional and intended for human
	// display.
	Report(progress float64, total *float64, message string) error
}

// progressReporterKey is an unexported context key type so the reporter slot
// does not collide with other ctx values.
type progressReporterKey struct{}

// WithProgressReporter returns ctx with r attached. The MCP server dispatcher
// uses this to hand the handler a reporter bound to the request's
// progressToken; callers outside the framework should not need it.
func WithProgressReporter(ctx context.Context, r ProgressReporter) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, progressReporterKey{}, r)
}

// ProgressReporterFromContext returns the reporter attached to ctx, or a
// no-op reporter if none is present. Never returns nil, so handlers can
// always call Report without a guard.
func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	if r, ok := ctx.Value(progressReporterKey{}).(ProgressReporter); ok && r != nil {
		return r
	}
	return noopReporter{}
}

// noopReporter drops every Report call. Used when no progressToken was
// supplied on the incoming request.
type noopReporter struct{}

func (noopReporter) Report(float64, *float64, string) error { return nil }
