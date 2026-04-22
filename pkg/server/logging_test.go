package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// TestInitializeAdvertisesLoggingCapability confirms the server's initialize
// response carries capabilities.logging so clients know they can call
// logging/setLevel.
func TestInitializeAdvertisesLoggingCapability(t *testing.T) {
	mockTransport := newMockTransport()
	srv := New(Options{
		Name:      "test-server",
		Version:   "1.0.0",
		Registry:  handler.NewHandlerRegistry(),
		Transport: mockTransport,
	})
	go srv.Run()

	params := map[string]interface{}{
		"protocolVersion": "2025-11-25",
		"clientInfo":      map[string]string{"name": "c", "version": "1"},
		"capabilities":    map[string]interface{}{},
	}
	paramsJSON, _ := json.Marshal(params)
	mockTransport.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodInitialize,
		Params:  paramsJSON,
	}

	time.Sleep(100 * time.Millisecond)

	if mockTransport.responseCount() == 0 {
		t.Fatal("no initialize response")
	}
	resp := mockTransport.responseAt(0)
	result, ok := resp.Result.(*protocol.InitializeResponse)
	if !ok {
		t.Fatalf("result is not InitializeResponse: %T", resp.Result)
	}
	if result.Capabilities.Logging == nil {
		t.Error("expected capabilities.logging to be advertised")
	}
}

// TestLoggingSetLevelUpdatesThreshold drives a logging/setLevel request and
// asserts the server returns an empty result and that subsequent LogMessage
// calls respect the new threshold.
func TestLoggingSetLevelUpdatesThreshold(t *testing.T) {
	mockTransport := newMockTransport()
	srv := New(Options{
		Name:      "test-server",
		Version:   "1.0.0",
		Registry:  handler.NewHandlerRegistry(),
		Transport: mockTransport,
	})
	go srv.Run()

	// Default level is info — debug should be filtered.
	if err := srv.LogMessage(protocol.LogLevelDebug, "tools", "before-setLevel"); err != nil {
		t.Fatalf("LogMessage debug: %v", err)
	}
	if mockTransport.notificationCount() != 0 {
		t.Errorf("debug message emitted at info threshold; notifications=%d", mockTransport.notificationCount())
	}

	// Lower threshold to debug.
	setParams, _ := json.Marshal(protocol.SetLevelParams{Level: protocol.LogLevelDebug})
	mockTransport.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  protocol.MethodLoggingSetLevel,
		Params:  setParams,
	}

	time.Sleep(100 * time.Millisecond)

	if mockTransport.responseCount() == 0 {
		t.Fatal("no logging/setLevel response")
	}
	resp := mockTransport.responseAt(0)
	if resp.Error != nil {
		t.Fatalf("setLevel returned error: %v", resp.Error)
	}

	// Debug message should now be emitted.
	if err := srv.LogMessage(protocol.LogLevelDebug, "tools", "after-setLevel"); err != nil {
		t.Fatalf("LogMessage debug: %v", err)
	}
	if mockTransport.notificationCount() != 1 {
		t.Fatalf("expected 1 notification after threshold drop, got %d", mockTransport.notificationCount())
	}
	mockTransport.mu.Lock()
	n := mockTransport.notifications[0]
	mockTransport.mu.Unlock()
	if n.Method != protocol.NotificationMessage {
		t.Errorf("method = %q, want %q", n.Method, protocol.NotificationMessage)
	}
	var got protocol.LogMessageParams
	if err := json.Unmarshal(n.Params, &got); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got.Level != protocol.LogLevelDebug {
		t.Errorf("level = %q, want debug", got.Level)
	}
	if got.Logger != "tools" {
		t.Errorf("logger = %q, want tools", got.Logger)
	}
	if s, _ := got.Data.(string); s != "after-setLevel" {
		t.Errorf("data = %v, want 'after-setLevel'", got.Data)
	}
}

// TestLoggingSetLevelRejectsUnknownLevel confirms an unknown level string
// yields an error response rather than silently accepting.
func TestLoggingSetLevelRejectsUnknownLevel(t *testing.T) {
	mockTransport := newMockTransport()
	srv := New(Options{
		Name:      "test-server",
		Version:   "1.0.0",
		Registry:  handler.NewHandlerRegistry(),
		Transport: mockTransport,
	})
	go srv.Run()

	setParams, _ := json.Marshal(map[string]string{"level": "bogus"})
	mockTransport.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodLoggingSetLevel,
		Params:  setParams,
	}

	time.Sleep(100 * time.Millisecond)

	if mockTransport.responseCount() == 0 {
		t.Fatal("no response")
	}
	resp := mockTransport.responseAt(0)
	if resp.Error == nil {
		t.Fatal("expected error response for unknown level")
	}
}
