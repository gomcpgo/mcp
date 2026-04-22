package handler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// ErrElicitationNotSupported is returned by the stub Elicitor when no real
// Elicitor has been injected (typically because the client did not advertise
// the elicitation capability during initialize).
var ErrElicitationNotSupported = errors.New("elicitation not supported by client")

// Elicitor is the handler-facing API for prompting the user with a dynamic
// form mid-tool-call. Handlers obtain one via ElicitorFromContext(ctx) and
// call Elicit. When the connected client does not support elicitation the
// returned Elicitor is a stub that errors with ErrElicitationNotSupported,
// so handlers can call Elicit unconditionally and surface the failure.
type Elicitor interface {
	// Elicit sends an elicitation/create request to the client and blocks
	// until the client responds, ctx is cancelled, or the framework's
	// default timeout fires. Returns the spec-defined ElicitationResult;
	// action is one of "accept" / "decline" / "cancel".
	Elicit(ctx context.Context, message string, requestedSchema json.RawMessage) (*protocol.ElicitationResult, error)
}

type elicitorKey struct{}

// WithElicitor stashes e in ctx so the tool handler can retrieve it via
// ElicitorFromContext. Passing a nil Elicitor returns ctx unchanged (the
// handler will then see the no-op stub).
func WithElicitor(ctx context.Context, e Elicitor) context.Context {
	if e == nil {
		return ctx
	}
	return context.WithValue(ctx, elicitorKey{}, e)
}

// ElicitorFromContext returns the Elicitor attached to ctx, or a stub that
// always errors with ErrElicitationNotSupported if none is present. Never
// returns nil, so handlers can always call Elicit without a guard.
func ElicitorFromContext(ctx context.Context) Elicitor {
	if e, ok := ctx.Value(elicitorKey{}).(Elicitor); ok && e != nil {
		return e
	}
	return unsupportedElicitor{}
}

type unsupportedElicitor struct{}

func (unsupportedElicitor) Elicit(context.Context, string, json.RawMessage) (*protocol.ElicitationResult, error) {
	return nil, ErrElicitationNotSupported
}
