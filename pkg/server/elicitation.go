package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// ErrElicitationNotSupported is returned by Server.Elicit when the connected
// client did not advertise the `elicitation` capability during initialize.
// Handlers that depend on elicitation should surface this to the LLM rather
// than silently substituting defaults.
var ErrElicitationNotSupported = errors.New("elicitation not supported by client")

// serverElicitor adapts *Server to the handler.Elicitor interface so the
// request dispatcher can inject the Server's Elicit method into handler ctx
// without the handler package importing server.
type serverElicitor struct {
	s *Server
}

func (e serverElicitor) Elicit(ctx context.Context, message string, requestedSchema json.RawMessage) (*protocol.ElicitationResult, error) {
	return e.s.Elicit(ctx, message, requestedSchema)
}

// defaultElicitationTimeout caps how long Server.Elicit waits for a client
// response when the caller has no ctx deadline of its own.
const defaultElicitationTimeout = 5 * time.Minute

// Elicit sends an elicitation/create request to the connected client and
// blocks until the client responds, ctx is cancelled, or the default 5-minute
// timeout fires (whichever the caller's ctx deadline or the default hits
// first). Only one elicitation may be in flight at a time per Server
// instance — concurrent callers serialize.
//
// Returns ErrElicitationNotSupported if the client did not advertise the
// elicitation capability. On ctx cancellation the server emits
// notifications/cancelled for the outbound request and returns ctx.Err().
func (s *Server) Elicit(
	ctx context.Context,
	message string,
	requestedSchema json.RawMessage,
) (*protocol.ElicitationResult, error) {
	if !s.clientSupportsElicitation() {
		return nil, ErrElicitationNotSupported
	}

	// Acquire the per-server mutex so only one elicitation is ever on the
	// wire. Respect ctx while waiting for the mutex so a cancelled caller
	// doesn't queue up behind a slow prior elicitation forever.
	if err := s.acquireElicitMu(ctx); err != nil {
		return nil, err
	}
	defer s.elicitMu.Unlock()

	// Build and send the request.
	id := s.outbound.nextID()
	params := protocol.ElicitationRequestParams{
		Message:         message,
		RequestedSchema: requestedSchema,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal elicitation params: %w", err)
	}
	req := &protocol.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  protocol.MethodElicitationCreate,
		Params:  paramsJSON,
	}

	waitCh := s.outbound.register(id)

	if err := s.transport.SendRequest(req); err != nil {
		s.outbound.cancel(id)
		return nil, fmt.Errorf("send elicitation request: %w", err)
	}

	// Apply the default timeout only if the caller didn't already set one.
	var timeoutCh <-chan time.Time
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timer := time.NewTimer(defaultElicitationTimeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case resp := <-waitCh:
		if resp == nil {
			// Tracker was cancelled out-of-band (server shutdown, etc.).
			return nil, errors.New("elicitation cancelled")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("client returned error: %s", resp.Error.Message)
		}
		return parseElicitationResult(resp)

	case <-ctx.Done():
		s.outbound.cancel(id)
		s.emitCancelled(id, ctx.Err())
		return nil, ctx.Err()

	case <-timeoutCh:
		s.outbound.cancel(id)
		s.emitCancelled(id, errors.New("elicitation timeout"))
		return nil, fmt.Errorf("elicitation timed out after %v", defaultElicitationTimeout)
	}
}

// clientSupportsElicitation reports whether the most recent initialize
// handshake declared elicitation support. Returns false if no initialize
// has happened yet.
func (s *Server) clientSupportsElicitation() bool {
	s.clientCapsMu.RLock()
	defer s.clientCapsMu.RUnlock()
	return s.clientCaps != nil && s.clientCaps.Elicitation != nil
}

// acquireElicitMu takes the elicitation mutex while honouring ctx.
func (s *Server) acquireElicitMu(ctx context.Context) error {
	acquired := make(chan struct{})
	go func() {
		s.elicitMu.Lock()
		close(acquired)
	}()
	select {
	case <-acquired:
		return nil
	case <-ctx.Done():
		// Race: if the lock was acquired just as ctx fired, release it so
		// we don't leak the mutex. The goroutine above will complete
		// and close(acquired), so wait for it before unlocking.
		go func() {
			<-acquired
			s.elicitMu.Unlock()
		}()
		return ctx.Err()
	}
}

// emitCancelled sends notifications/cancelled for an outbound server→client
// request the server has abandoned (timeout or ctx cancel).
func (s *Server) emitCancelled(id interface{}, cause error) {
	reason := "cancelled"
	if cause != nil {
		reason = cause.Error()
	}
	if err := s.SendNotification(protocol.NotificationCancelled, protocol.CancelledParams{
		RequestID: id,
		Reason:    reason,
	}); err != nil {
		log.Printf("failed to emit notifications/cancelled for outbound id=%v: %v", id, err)
	}
}

// parseElicitationResult extracts the ElicitationResult from a JSON-RPC
// response. Tolerates both an already-marshalled RawMessage and a direct
// *protocol.ElicitationResult in Result (the mock transport sometimes sends
// the latter).
func parseElicitationResult(resp *protocol.Response) (*protocol.ElicitationResult, error) {
	switch v := resp.Result.(type) {
	case nil:
		return &protocol.ElicitationResult{Action: protocol.ElicitationActionCancel}, nil
	case json.RawMessage:
		var r protocol.ElicitationResult
		if err := json.Unmarshal(v, &r); err != nil {
			return nil, fmt.Errorf("parse elicitation result: %w", err)
		}
		return &r, nil
	case []byte:
		var r protocol.ElicitationResult
		if err := json.Unmarshal(v, &r); err != nil {
			return nil, fmt.Errorf("parse elicitation result: %w", err)
		}
		return &r, nil
	default:
		// Re-marshal and re-parse — covers maps, structs, etc.
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal elicitation result for reparse: %w", err)
		}
		var r protocol.ElicitationResult
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("parse elicitation result: %w", err)
		}
		return &r, nil
	}
}
