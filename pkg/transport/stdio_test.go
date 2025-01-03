package transport

import (
	"context"
	"encoding/json"
	"io"
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
	if t.inReader != nil {
		t.inReader.Close()
	}
	if t.inWriter != nil {
		t.inWriter.Close()
	}
	if t.outReader != nil {
		t.outReader.Close()
	}
	if t.outWriter != nil {
		t.outWriter.Close()
	}
}

func TestStdioTransport(t *testing.T) {
	tp, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer tp.cleanup()

	transport := &StdioTransport{
		encoder:  json.NewEncoder(tp.outWriter),
		decoder:  json.NewDecoder(tp.inReader),
		requests: make(chan *protocol.Request, 10), // Buffered channels
		errors:   make(chan error, 10),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Test valid request
	testRequest := &protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  json.RawMessage(`{"test":"value"}`),
	}

	if err := json.NewEncoder(tp.inWriter).Encode(testRequest); err != nil {
		t.Fatalf("Failed to write test request: %v", err)
	}

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

	// Test invalid request
	if _, err := io.WriteString(tp.inWriter, "invalid json\n"); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	select {
	case err := <-transport.Errors():
		if !strings.Contains(err.Error(), "decode error") {
			t.Errorf("got error = %v, want decode error", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error")
	}

	// Test response
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "test result",
	}

	if err := transport.Send(resp); err != nil {
		t.Errorf("Send() error = %v", err)
	}

	// Test graceful shutdown
	if err := transport.Stop(ctx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Test send after shutdown
	if err := transport.Send(resp); err == nil {
		t.Error("Send() should return error after shutdown")
	} else if !strings.Contains(err.Error(), "transport is closed") {
		t.Errorf("expected 'transport is closed' error, got: %v", err)
	}

	// Wait for channels to close
	time.Sleep(50 * time.Millisecond)

	// Try reading from closed channels
	select {
	case _, ok := <-transport.Receive():
		if ok {
			t.Error("request channel should be closed")
		}
	default:
		t.Error("request channel should be closed")
	}

	select {
	case _, ok := <-transport.Errors():
		if ok {
			t.Error("error channel should be closed")
		}
	default:
		t.Error("error channel should be closed")
	}
}

func TestTransportEOF(t *testing.T) {
	tp, err := setupTestTransport()
	if err != nil {
		t.Fatalf("Failed to set up test transport: %v", err)
	}
	defer tp.cleanup()

	transport := &StdioTransport{
		encoder:  json.NewEncoder(tp.outWriter),
		decoder:  json.NewDecoder(tp.inReader),
		requests: make(chan *protocol.Request, 10),
		errors:   make(chan error, 10),
		done:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Close input to simulate EOF
	tp.inWriter.Close()

	// Wait for EOF to be processed
	time.Sleep(50 * time.Millisecond)

	// Try to send after EOF
	if err := transport.Send(&protocol.Response{}); err == nil {
		t.Error("Send() should return error after EOF")
	}
}
