package transport

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

type testTransport struct {
	inReader  *os.File
	inWriter  *os.File
	outReader *os.File
	outWriter *os.File
}

func setupTestTransport() (*testTransport, error) {
	// Create pipes for stdin and stdout
	inReader, inWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		inReader.Close()
		inWriter.Close()
		return nil, err
	}

	return &testTransport{
		inReader:  inReader,
		inWriter:  inWriter,
		outReader: outReader,
		outWriter: outWriter,
	}, nil
}

func (t *testTransport) cleanup() {
	t.inReader.Close()
	t.inWriter.Close()
	t.outReader.Close()
	t.outWriter.Close()
}

func TestStdioTransport(t *testing.T) {
	tp, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer tp.cleanup()

	// Create transport
	transport := &StdioTransport{
		encoder:  json.NewEncoder(tp.outWriter),
		decoder:  json.NewDecoder(tp.inReader),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}

	// Start transport with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Test sending a request
	testRequest := &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  json.RawMessage(`{"test":"value"}`),
	}

	go func() {
		if err := json.NewEncoder(tp.inWriter).Encode(testRequest); err != nil {
			t.Errorf("Failed to write test request: %v", err)
		}
	}()

	// Wait for request or timeout
	select {
	case req := <-transport.Receive():
		if req.JSONRPC != testRequest.JSONRPC {
			t.Errorf("got JSONRPC = %v, want %v", req.JSONRPC, testRequest.JSONRPC)
		}
		if req.Method != testRequest.Method {
			t.Errorf("got Method = %v, want %v", req.Method, testRequest.Method)
		}
	case err := <-transport.Errors():
		t.Fatalf("got error instead of request: %v", err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for request")
	}

	// Test invalid JSON
	go func() {
		if _, err := tp.inWriter.Write([]byte("invalid json\n")); err != nil {
			t.Errorf("Failed to write invalid JSON: %v", err)
		}
	}()

	// Wait for error or timeout
	select {
	case err := <-transport.Errors():
		if !strings.Contains(err.Error(), "decode error") {
			t.Errorf("got error = %v, want decode error", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error")
	}

	// Test shutdown
	cancel()
	time.Sleep(50 * time.Millisecond) // Give time for shutdown

	// Verify channels are closed
	select {
	case _, ok := <-transport.Receive():
		if ok {
			t.Error("receive channel should be closed")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("timeout waiting for receive channel to close")
	}

	select {
	case _, ok := <-transport.Errors():
		if ok {
			t.Error("errors channel should be closed")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("timeout waiting for errors channel to close")
	}
}

func TestTransportShutdown(t *testing.T) {
	tp, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer tp.cleanup()

	transport := &StdioTransport{
		encoder:  json.NewEncoder(tp.outWriter),
		decoder:  json.NewDecoder(tp.inReader),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Test clean shutdown
	cancel()
	time.Sleep(50 * time.Millisecond)

	if err := transport.Send(&protocol.Response{}); err == nil {
		t.Error("Send() should return error after shutdown")
	}
}
