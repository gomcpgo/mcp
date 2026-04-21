package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// initializeWithCaps sends an initialize request over the mock transport
// with the given client capabilities and waits for the server to record
// them. Returns the server.
func initializeWithCaps(t *testing.T, elicitationSupported bool) (*Server, *mockTransport) {
	t.Helper()
	mt := newMockTransport()
	registry := handler.NewHandlerRegistry()
	srv := New(Options{
		Name:      "test-server",
		Version:   "1.0.0",
		Registry:  registry,
		Transport: mt,
	})
	go srv.Run()

	caps := map[string]interface{}{}
	if elicitationSupported {
		caps["elicitation"] = map[string]interface{}{}
	}
	params := map[string]interface{}{
		"protocolVersion": "2025-11-25",
		"clientInfo":      map[string]string{"name": "test", "version": "1"},
		"capabilities":    caps,
	}
	paramsJSON, _ := json.Marshal(params)
	mt.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodInitialize,
		Params:  paramsJSON,
	}

	// Wait for the initialize response to be captured.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mt.responseCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return srv, mt
}

func TestElicit_ErrorsWhenClientLacksCapability(t *testing.T) {
	srv, _ := initializeWithCaps(t, false)

	_, err := srv.Elicit(context.Background(), "hi", json.RawMessage(`{"type":"object"}`))
	if err == nil {
		t.Fatal("expected error when client did not advertise elicitation")
	}
	if !errors.Is(err, ErrElicitationNotSupported) {
		t.Errorf("err = %v, want ErrElicitationNotSupported", err)
	}
}

func TestElicit_SendsRequestAndReturnsResult(t *testing.T) {
	srv, mt := initializeWithCaps(t, true)

	// Call Elicit in a goroutine; feed a matching response on the mock
	// transport so it unblocks.
	var (
		result *protocol.ElicitationResult
		err    error
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err = srv.Elicit(
			context.Background(),
			"Please confirm",
			json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
		)
	}()

	// Wait for the outbound request to land.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mt.outboundRequestCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mt.outboundRequestCount() == 0 {
		t.Fatal("no outbound request captured")
	}
	req := mt.outboundRequestAt(0)
	if req.Method != protocol.MethodElicitationCreate {
		t.Errorf("method = %q, want %q", req.Method, protocol.MethodElicitationCreate)
	}

	// Feed a response with the matching ID.
	resultJSON, _ := json.Marshal(protocol.ElicitationResult{
		Action:  protocol.ElicitationActionAccept,
		Content: map[string]interface{}{"ok": true},
	})
	mt.clientResps <- &protocol.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  json.RawMessage(resultJSON),
	}

	wg.Wait()
	if err != nil {
		t.Fatalf("Elicit returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Action != protocol.ElicitationActionAccept {
		t.Errorf("action = %q, want accept", result.Action)
	}
	if v, ok := result.Content["ok"].(bool); !ok || !v {
		t.Errorf("content.ok = %v, want true", result.Content["ok"])
	}
}

func TestElicit_ContextCancelEmitsNotificationCancelled(t *testing.T) {
	srv, mt := initializeWithCaps(t, true)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var callErr error
	go func() {
		_, callErr = srv.Elicit(ctx, "", json.RawMessage(`{"type":"object"}`))
		close(done)
	}()

	// Wait for the request to be on the wire, then cancel.
	for mt.outboundRequestCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Elicit did not return after ctx cancel")
	}

	if !errors.Is(callErr, context.Canceled) {
		t.Errorf("callErr = %v, want context.Canceled", callErr)
	}

	// Expect a notifications/cancelled emitted for the request id.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mt.notificationCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mt.notificationCount() == 0 {
		t.Fatal("no cancellation notification emitted")
	}
	mt.mu.Lock()
	defer mt.mu.Unlock()
	var found bool
	for _, n := range mt.notifications {
		if n.Method == protocol.NotificationCancelled {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected notifications/cancelled among emitted notifications")
	}
}

func TestElicit_OneAtATime(t *testing.T) {
	srv, mt := initializeWithCaps(t, true)

	// Fire two concurrent Elicit calls; the first must complete before the
	// second's request even goes out.
	type outcome struct {
		action string
		err    error
	}
	out := make(chan outcome, 2)

	go func() {
		r, err := srv.Elicit(context.Background(), "first", json.RawMessage(`{}`))
		if r != nil {
			out <- outcome{r.Action, err}
		} else {
			out <- outcome{"", err}
		}
	}()

	// Give the first a head start to grab the mutex.
	time.Sleep(30 * time.Millisecond)

	go func() {
		r, err := srv.Elicit(context.Background(), "second", json.RawMessage(`{}`))
		if r != nil {
			out <- outcome{r.Action, err}
		} else {
			out <- outcome{"", err}
		}
	}()

	// Only one outbound request should be on the wire before the first is answered.
	time.Sleep(50 * time.Millisecond)
	if n := mt.outboundRequestCount(); n != 1 {
		t.Fatalf("expected 1 outbound request while first is in flight, got %d", n)
	}

	// Answer the first. Its id is the only outbound request so far.
	first := mt.outboundRequestAt(0)
	firstResult, _ := json.Marshal(protocol.ElicitationResult{Action: protocol.ElicitationActionAccept})
	mt.clientResps <- &protocol.Response{JSONRPC: "2.0", ID: first.ID, Result: firstResult}

	// Now the second should issue its outbound request; answer it.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if mt.outboundRequestCount() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mt.outboundRequestCount() < 2 {
		t.Fatalf("second elicitation never issued; count=%d", mt.outboundRequestCount())
	}
	second := mt.outboundRequestAt(1)
	secondResult, _ := json.Marshal(protocol.ElicitationResult{Action: protocol.ElicitationActionDecline})
	mt.clientResps <- &protocol.Response{JSONRPC: "2.0", ID: second.ID, Result: secondResult}

	// Collect outcomes.
	for i := 0; i < 2; i++ {
		select {
		case <-out:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for Elicit to return")
		}
	}
}
