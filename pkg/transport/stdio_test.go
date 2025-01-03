package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

type testTransport struct {
	stdin  *os.File
	stdout *os.File
}

func setupTestTransport() (*testTransport, *bytes.Buffer, func(), error) {
	// Create pipes for stdin and stdout
	inReader, inWriter, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		inReader.Close()
		inWriter.Close()
		return nil, nil, nil, err
	}

	// Create a buffer to capture output
	outBuf := &bytes.Buffer{}

	// Create cleanup function
	cleanup := func() {
		inReader.Close()
		inWriter.Close()
		outReader.Close()
		outWriter.Close()
	}

	return &testTransport{
		stdin:  inReader,
		stdout: outWriter,
	}, outBuf, cleanup, nil
}

func TestStdioTransport(t *testing.T) {
	// Set up test transport
	testTransport, _, cleanup, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer cleanup()

	// Create transport with test pipes
	transport := &StdioTransport{
		encoder:  json.NewEncoder(testTransport.stdout),
		decoder:  json.NewDecoder(testTransport.stdin),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}

	// Start transport
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Test sending a request
	testRequest := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  json.RawMessage(`{"test":"value"}`),
	}

	// Write request to pipe
	encoder := json.NewEncoder(testTransport.stdout)
	if err := encoder.Encode(testRequest); err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	// Wait for request
	select {
	case req := <-transport.Receive():
		if req.JSONRPC != testRequest.JSONRPC {
			t.Errorf("Received request JSONRPC = %v, want %v", req.JSONRPC, testRequest.JSONRPC)
		}
		if req.Method != testRequest.Method {
			t.Errorf("Received request Method = %v, want %v", req.Method, testRequest.Method)
		}
	case err := <-transport.Errors():
		t.Fatalf("Received error instead of request: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for request")
	}

	// Test sending a response
	testResponse := &protocol.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]interface{}{"test": "value"},
	}

	if err := transport.Send(testResponse); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Wait for response to be written
	time.Sleep(100 * time.Millisecond)

	// Test invalid request
	if _, err := testTransport.stdout.Write([]byte("invalid json\n")); err != nil {
		t.Fatalf("Failed to write invalid request: %v", err)
	}

	// Should receive an error
	select {
	case err := <-transport.Errors():
		if !strings.Contains(err.Error(), "decode error") {
			t.Errorf("Expected decode error, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for error")
	}

	// Test wrong JSON-RPC version
	wrongVersionReq := protocol.Request{
		JSONRPC: "1.0",
		ID:      2,
		Method:  "test",
	}
	if err := encoder.Encode(wrongVersionReq); err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	// Should receive an error
	select {
	case err := <-transport.Errors():
		if !strings.Contains(err.Error(), "invalid JSON-RPC version") {
			t.Errorf("Expected version error, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for error")
	}

	// Test transport shutdown
	if err := transport.Stop(ctx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Write after stop should not panic
	if err := transport.Send(testResponse); err == nil {
		t.Error("Expected error when sending after stop")
	}
}

func TestStdioTransportContextCancellation(t *testing.T) {
	testTransport, _, cleanup, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer cleanup()

	transport := &StdioTransport{
		encoder:  json.NewEncoder(testTransport.stdout),
		decoder:  json.NewDecoder(testTransport.stdin),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}

	// Start transport with cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Cancel context
	cancel()

	// Give the transport time to shut down
	time.Sleep(100 * time.Millisecond)

	// Write after context cancellation should not panic
	if err := transport.Send(&protocol.Response{}); err == nil {
		t.Error("Expected error when sending after context cancellation")
	}

	// Ensure transport has stopped
	select {
	case _, ok := <-transport.Receive():
		if ok {
			t.Error("Expected receive channel to be closed")
		}
	default:
		t.Error("Expected receive channel to be closed")
	}
}

func TestStdioTransportEOF(t *testing.T) {
	testTransport, _, cleanup, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer cleanup()

	// Close stdin pipe to simulate EOF
	testTransport.stdin.Close()

	transport := &StdioTransport{
		encoder:  json.NewEncoder(testTransport.stdout),
		decoder:  json.NewDecoder(testTransport.stdin),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}

	// Start transport
	ctx := context.Background()
	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give the transport time to process EOF
	time.Sleep(100 * time.Millisecond)

	// Verify channels are closed on EOF
	if _, ok := <-transport.Receive(); ok {
		t.Error("Expected receive channel to be closed on EOF")
	}
}
