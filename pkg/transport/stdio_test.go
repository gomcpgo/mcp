package transport

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

/*
func TestStdioTransport(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	os.Stdin = pr
	os.Stdout = pw

	transport := NewStdioTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Test valid request
	testRequest := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  json.RawMessage(`{"test":"value"}`),
	}

	if err := json.NewEncoder(pw).Encode(testRequest); err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	select {
	case req := <-transport.Receive():
		if req.JSONRPC != testRequest.JSONRPC {
			t.Errorf("got JSONRPC = %v, want %v", req.JSONRPC, testRequest.JSONRPC)
		}
	case err := <-transport.Errors():
		t.Fatalf("got error instead of request: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for request")
	}

	// Test invalid JSON
	if _, err := pw.WriteString("invalid json\n"); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	select {
	case err := <-transport.Errors():
		if !strings.Contains(err.Error(), "decode error") {
			t.Errorf("got error = %v, want decode error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error")
	}

	// Test shutdown
	cancel()
	time.Sleep(100 * time.Millisecond)

	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "test",
	}

	if err := transport.Send(resp); err == nil {
		t.Error("Send() should return error after shutdown")
	}
}
*/

func TestEOF(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	os.Stdin = pr
	os.Stdout = pw

	transport := NewStdioTransport()
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Close write end to simulate EOF
	pw.Close()

	// Wait for transport to handle EOF
	time.Sleep(100 * time.Millisecond)

	// Channel should be closed
	if _, ok := <-transport.Receive(); ok {
		t.Error("channel should be closed after EOF")
	}
}

func TestContextCancellation(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	os.Stdin = pr
	os.Stdout = pw

	transport := NewStdioTransport()
	ctx, cancel := context.WithCancel(context.Background())

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "test",
	}

	if err := transport.Send(resp); err == nil {
		t.Error("Send() should return error after context cancellation")
	}
}
