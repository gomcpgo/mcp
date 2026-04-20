package server

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// cancellingToolHandler is a ToolHandler whose CallTool blocks until ctx is
// cancelled (or a timeout fires). Used to exercise the server-side
// cancellation dispatch path.
type cancellingToolHandler struct {
	ctxSeen      chan context.Context
	stillWaiting atomic.Bool
	// ignoreCtx — if set, CallTool does not observe ctx.Done(); it always
	// returns after a short delay so we can verify the framework suppresses
	// the response when the client cancelled.
	ignoreCtx bool
	// returnDelay controls how long CallTool waits before returning in
	// ignoreCtx mode. In normal mode it's an upper bound.
	returnDelay time.Duration
}

func (h *cancellingToolHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	return &protocol.ListToolsResponse{Tools: []protocol.Tool{}}, nil
}

func (h *cancellingToolHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	if h.ctxSeen != nil {
		select {
		case h.ctxSeen <- ctx:
		default:
		}
	}
	h.stillWaiting.Store(true)
	defer h.stillWaiting.Store(false)

	if h.ignoreCtx {
		time.Sleep(h.returnDelay)
		return &protocol.CallToolResponse{
			Content: []protocol.ToolContent{{Type: "text", Text: "done after delay"}},
		}, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(h.returnDelay):
		return &protocol.CallToolResponse{
			Content: []protocol.ToolContent{{Type: "text", Text: "completed normally"}},
		}, nil
	}
}

// buildServerWithHandler wires a mock transport + registered tool handler
// and starts the server loop in a goroutine.
func buildServerWithHandler(t *testing.T, tool *cancellingToolHandler) *mockTransport {
	t.Helper()
	transp := newMockTransport()
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(tool)
	srv := New(Options{
		Name:      "cancel-test-server",
		Version:   "1.0.0",
		Registry:  registry,
		Transport: transp,
	})
	go srv.Run()
	return transp
}

// sendCall is a small helper that injects a tools/call request with the
// given id and returns after the transport receives it.
func sendCall(t *testing.T, transp *mockTransport, id interface{}) {
	t.Helper()
	params, _ := json.Marshal(map[string]interface{}{
		"name":      "test-tool",
		"arguments": map[string]interface{}{},
	})
	transp.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  protocol.MethodToolsCall,
		Params:  params,
	}
}

// sendCancellation injects a notifications/cancelled message targeting id.
func sendCancellation(t *testing.T, transp *mockTransport, id interface{}, reason string) {
	t.Helper()
	params, _ := json.Marshal(map[string]interface{}{
		"requestId": id,
		"reason":    reason,
	})
	transp.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      nil, // notification
		Method:  protocol.NotificationCancelled,
		Params:  params,
	}
}

// TestHandleRequest_CancellationCancelsCtx verifies that an inbound
// notifications/cancelled fires Done on the handler's context.
func TestHandleRequest_CancellationCancelsCtx(t *testing.T) {
	tool := &cancellingToolHandler{
		ctxSeen:     make(chan context.Context, 1),
		returnDelay: 3 * time.Second, // well beyond the cancellation wait
	}
	transp := buildServerWithHandler(t, tool)

	sendCall(t, transp, 101)

	// Grab the handler's ctx once the handler is running.
	var handlerCtx context.Context
	select {
	case handlerCtx = <-tool.ctxSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CallTool was not invoked within 500ms")
	}

	sendCancellation(t, transp, 101, "user cancelled")

	select {
	case <-handlerCtx.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler ctx was not cancelled after notifications/cancelled")
	}

	// Give time for handler to return and response suppression to run.
	time.Sleep(100 * time.Millisecond)

	if transp.responseCount() != 0 {
		t.Errorf("server sent %d responses for a cancelled request; want 0",
			transp.responseCount())
	}
}

// TestHandleRequest_ResponseSuppressedAfterCancellation covers the case where
// the handler ignores ctx.Done() and returns a result anyway — the server must
// still suppress the response because the client already cancelled.
func TestHandleRequest_ResponseSuppressedAfterCancellation(t *testing.T) {
	tool := &cancellingToolHandler{
		ctxSeen:     make(chan context.Context, 1),
		ignoreCtx:   true,
		returnDelay: 300 * time.Millisecond,
	}
	transp := buildServerWithHandler(t, tool)

	sendCall(t, transp, "req-a")

	// Wait for handler to actually start so cancellation finds the ID in-flight.
	select {
	case <-tool.ctxSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CallTool was not invoked")
	}

	sendCancellation(t, transp, "req-a", "abandoned")

	// Handler sleeps 300ms then returns; give it + suppression time.
	time.Sleep(500 * time.Millisecond)

	if transp.responseCount() != 0 {
		t.Errorf("server sent %d responses after cancellation; want 0 (handler ignored ctx)",
			transp.responseCount())
	}
}

// TestHandleRequest_CancellationAfterCompletionIsNoop sends a cancellation
// for an ID that has already completed. The server should not panic, should
// not send anything, and should leave subsequent requests unaffected.
func TestHandleRequest_CancellationAfterCompletionIsNoop(t *testing.T) {
	tool := &cancellingToolHandler{
		ctxSeen:     make(chan context.Context, 2),
		returnDelay: 50 * time.Millisecond,
	}
	transp := buildServerWithHandler(t, tool)

	sendCall(t, transp, 1)

	// Wait until the response lands.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if transp.responseCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if transp.responseCount() != 1 {
		t.Fatalf("expected 1 response, got %d", transp.responseCount())
	}

	// Now send cancellation for the already-completed ID.
	sendCancellation(t, transp, 1, "late")

	// Send another request; it should be handled normally.
	sendCall(t, transp, 2)
	time.Sleep(150 * time.Millisecond)

	if transp.responseCount() < 2 {
		t.Fatalf("second request not handled; responses=%d", transp.responseCount())
	}
	if id := transp.responsesSnapshot()[1].ID; id != 2 {
		t.Errorf("second response id = %v, want 2", id)
	}
}

// TestNotificationCancelled_UnknownRequestIDIgnored sends a cancellation for
// a request ID the server never issued. Must not panic, must not respond.
func TestNotificationCancelled_UnknownRequestIDIgnored(t *testing.T) {
	tool := &cancellingToolHandler{returnDelay: 50 * time.Millisecond}
	transp := buildServerWithHandler(t, tool)

	sendCancellation(t, transp, 9999, "nope")

	time.Sleep(100 * time.Millisecond)

	if transp.responseCount() != 0 {
		t.Errorf("server sent %d responses for an unknown-ID cancellation; want 0",
			transp.responseCount())
	}
}

// TestRequestTracker_Concurrent exercises the server-side request tracker
// under concurrent register/cancel/unregister traffic to ensure
// go test -race stays clean.
func TestRequestTracker_Concurrent(t *testing.T) {
	tracker := newRequestTracker()

	const workers = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = tracker.register(context.Background(), id)
			tracker.cancel(id)
			tracker.unregister(id)
		}(i)
	}

	for i := workers; i < workers*2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = tracker.register(context.Background(), id)
			tracker.unregister(id)
		}(i)
	}

	wg.Wait()
}
