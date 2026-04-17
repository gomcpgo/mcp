package transport

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

func TestStdioSendNotification(t *testing.T) {
	// Capture stdout via pipe
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = pw
	defer func() { os.Stdout = oldStdout }()

	transport := NewStdioTransport()

	notif, err := protocol.NewNotification("notifications/tools/list_changed", nil)
	if err != nil {
		t.Fatalf("NewNotification: %v", err)
	}

	if err := transport.SendNotification(notif); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}

	// Close write end so reader sees EOF after the single message
	pw.Close()
	data, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal output %q: %v", string(data), err)
	}

	if _, hasID := decoded["id"]; hasID {
		t.Errorf("notification output must not contain id field: %s", string(data))
	}
	if decoded["method"] != "notifications/tools/list_changed" {
		t.Errorf("method = %v, want notifications/tools/list_changed", decoded["method"])
	}
	if decoded["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", decoded["jsonrpc"])
	}
}

func TestStdioSendNotificationAfterClose(t *testing.T) {
	transport := NewStdioTransport()
	_ = transport.Stop(context.Background())

	notif, _ := protocol.NewNotification("notifications/initialized", nil)
	if err := transport.SendNotification(notif); err == nil {
		t.Error("SendNotification should return error after transport is closed")
	}
}

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
