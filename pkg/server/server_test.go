package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Mock transport for testing
type mockTransport struct {
	requests  chan *protocol.Request
	errors    chan error
	responses []*protocol.Response
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		requests:  make(chan *protocol.Request, 10),
		errors:    make(chan error, 10),
		responses: make([]*protocol.Response, 0),
	}
}

func (t *mockTransport) Start(ctx context.Context) error {
	return nil
}

func (t *mockTransport) Stop(ctx context.Context) error {
	close(t.requests)
	close(t.errors)
	return nil
}

func (t *mockTransport) Send(response *protocol.Response) error {
	t.responses = append(t.responses, response)
	return nil
}

func (t *mockTransport) Receive() <-chan *protocol.Request {
	return t.requests
}

func (t *mockTransport) Errors() <-chan error {
	return t.errors
}

// Mock handlers for testing
type mockToolHandler struct {
	tools  []protocol.Tool
	result *protocol.CallToolResponse
}

func (h *mockToolHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	return &protocol.ListToolsResponse{Tools: h.tools}, nil
}

func (h *mockToolHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	return h.result, nil
}

func TestServer(t *testing.T) {
	// Create mock transport
	mockTransport := newMockTransport()

	// Create mock tool handler
	mockTools := []protocol.Tool{
		{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: json.RawMessage(`{}`),
		},
	}
	mockToolHandler := &mockToolHandler{
		tools: mockTools,
		result: &protocol.CallToolResponse{
			Content: []protocol.ToolContent{
				{Type: "text", Text: "test result"},
			},
		},
	}

	// Create registry and register handler
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(mockToolHandler)

	// Create server
	srv := New(Options{
		Name:      "test-server",
		Version:   "1.0.0",
		Registry:  registry,
		Transport: mockTransport,
	})

	// Start server in background
	errCh := make(chan error)
	go func() {
		errCh <- srv.Run()
	}()

	// Test initialize request
	initReq := &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodInitialize,
		Params:  json.RawMessage(`{}`),
	}
	mockTransport.requests <- initReq

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify initialize response
	if len(mockTransport.responses) == 0 {
		t.Fatal("no response received")
	}
	initResp := mockTransport.responses[0]
	if initResp.ID != initReq.ID {
		t.Errorf("initialize response ID = %v, want %v", initResp.ID, initReq.ID)
	}

	// Test list tools request
	toolsReq := &protocol.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  protocol.MethodToolsList,
	}
	mockTransport.requests <- toolsReq

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify list tools response
	if len(mockTransport.responses) < 2 {
		t.Fatal("no tools response received")
	}
	toolsResp := mockTransport.responses[1]
	if toolsResp.ID != toolsReq.ID {
		t.Errorf("tools response ID = %v, want %v", toolsResp.ID, toolsReq.ID)
	}

	// Test call tool request
	callReq := &protocol.Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  protocol.MethodToolsCall,
		Params: json.RawMessage(`{
			"name": "test-tool",
			"arguments": {"test": "value"}
		}`),
	}
	mockTransport.requests <- callReq

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify call tool response
	if len(mockTransport.responses) < 3 {
		t.Fatal("no call tool response received")
	}
	callResp := mockTransport.responses[2]
	if callResp.ID != callReq.ID {
		t.Errorf("call tool response ID = %v, want %v", callResp.ID, callReq.ID)
	}

	// Test invalid request
	invalidReq := &protocol.Request{
		JSONRPC: "1.0", // Invalid version
		ID:      4,
		Method:  "test",
	}
	mockTransport.requests <- invalidReq

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify error response
	if len(mockTransport.responses) < 4 {
		t.Fatal("no error response received")
	}
	errorResp := mockTransport.responses[3]
	if errorResp.ID != invalidReq.ID {
		t.Errorf("error response ID = %v, want %v", errorResp.ID, invalidReq.ID)
	}
	if errorResp.Error == nil {
		t.Error("expected error response")
	}

	// Test unknown method
	unknownReq := &protocol.Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "unknown",
	}
	mockTransport.requests <- unknownReq

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify error response
	if len(mockTransport.responses) < 5 {
		t.Fatal("no unknown method response received")
	}
	unknownResp := mockTransport.responses[4]
	if unknownResp.ID != unknownReq.ID {
		t.Errorf("unknown method response ID = %v, want %v", unknownResp.ID, unknownReq.ID)
	}
	if unknownResp.Error == nil {
		t.Error("expected error response for unknown method")
	}
}
