package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// progressEmittingToolHandler is a ToolHandler that invokes ProgressReporter
// from ctx with a fixed sequence and records what it did for test assertions.
type progressEmittingToolHandler struct {
	reported bool
}

func (h *progressEmittingToolHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	return &protocol.ListToolsResponse{Tools: []protocol.Tool{}}, nil
}

func (h *progressEmittingToolHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	r := handler.ProgressReporterFromContext(ctx)
	total := 100.0
	if err := r.Report(25, &total, "quarter"); err != nil {
		return nil, err
	}
	if err := r.Report(50, &total, "half"); err != nil {
		return nil, err
	}
	h.reported = true
	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{{Type: "text", Text: "done"}},
	}, nil
}

func buildServerWithProgressTool(t *testing.T) (*mockTransport, *progressEmittingToolHandler) {
	t.Helper()
	transp := newMockTransport()
	tool := &progressEmittingToolHandler{}
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(tool)
	srv := New(Options{
		Name:      "progress-test-server",
		Version:   "1.0.0",
		Registry:  registry,
		Transport: transp,
	})
	go srv.Run()
	return transp, tool
}

func sendToolCallWithProgressToken(t *testing.T, transp *mockTransport, id interface{}, token interface{}) {
	t.Helper()
	params := map[string]interface{}{
		"name":      "test-tool",
		"arguments": map[string]interface{}{},
	}
	if token != nil {
		params["_meta"] = map[string]interface{}{"progressToken": token}
	}
	paramsJSON, _ := json.Marshal(params)
	transp.requests <- &protocol.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  protocol.MethodToolsCall,
		Params:  paramsJSON,
	}
}

// TestHandleRequest_ProgressReporterEmitsWhenTokenPresent verifies that a
// request carrying a `_meta.progressToken` causes the handler's reporter
// calls to become outbound notifications/progress messages.
func TestHandleRequest_ProgressReporterEmitsWhenTokenPresent(t *testing.T) {
	transp, tool := buildServerWithProgressTool(t)

	sendToolCallWithProgressToken(t, transp, 1, "tok-xyz")

	// Wait briefly for the handler to run.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if tool.reported && transp.responseCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !tool.reported {
		t.Fatal("tool handler did not run")
	}

	notifs := drainNotifications(transp)
	if len(notifs) != 2 {
		t.Fatalf("expected 2 progress notifications, got %d", len(notifs))
	}
	for i, n := range notifs {
		if n.Method != protocol.NotificationProgress {
			t.Errorf("notif[%d].Method = %q, want %q", i, n.Method, protocol.NotificationProgress)
		}
		var params protocol.ProgressParams
		if err := json.Unmarshal(n.Params, &params); err != nil {
			t.Fatalf("notif[%d] params unmarshal: %v", i, err)
		}
		if params.ProgressToken != "tok-xyz" {
			t.Errorf("notif[%d].ProgressToken = %v, want tok-xyz", i, params.ProgressToken)
		}
	}
	if p0 := mustProgressParams(t, notifs[0].Params); p0.Progress != 25 {
		t.Errorf("progress[0] = %v, want 25", p0.Progress)
	}
	if p1 := mustProgressParams(t, notifs[1].Params); p1.Progress != 50 {
		t.Errorf("progress[1] = %v, want 50", p1.Progress)
	}
}

// TestHandleRequest_NoTokenMakesReporterNoop verifies that a tool call
// without a progressToken causes the reporter to drop all Report calls —
// no outbound notifications/progress is emitted.
func TestHandleRequest_NoTokenMakesReporterNoop(t *testing.T) {
	transp, tool := buildServerWithProgressTool(t)

	sendToolCallWithProgressToken(t, transp, 1, nil)

	// Wait for the handler to run.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if tool.reported && transp.responseCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !tool.reported {
		t.Fatal("tool handler did not run")
	}

	notifs := drainNotifications(transp)
	for _, n := range notifs {
		if n.Method == protocol.NotificationProgress {
			t.Errorf("no-token path should not emit progress; got %+v", n)
		}
	}
}

// TestHandleRequest_NumericProgressTokenPreserved verifies that a numeric
// token (JSON number in the request) flows through the reporter unchanged.
func TestHandleRequest_NumericProgressTokenPreserved(t *testing.T) {
	transp, _ := buildServerWithProgressTool(t)

	sendToolCallWithProgressToken(t, transp, 1, 42)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if transp.responseCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	notifs := drainNotifications(transp)
	if len(notifs) == 0 {
		t.Fatal("expected progress notifications")
	}
	p := mustProgressParams(t, notifs[0].Params)
	// JSON unmarshals numbers into float64.
	if got, ok := p.ProgressToken.(float64); !ok || got != 42 {
		t.Errorf("ProgressToken = %v (%T), want 42 (float64)", p.ProgressToken, p.ProgressToken)
	}
}

func drainNotifications(transp *mockTransport) []*protocol.Notification {
	transp.mu.Lock()
	defer transp.mu.Unlock()
	out := make([]*protocol.Notification, len(transp.notifications))
	copy(out, transp.notifications)
	return out
}

func mustProgressParams(t *testing.T, raw json.RawMessage) protocol.ProgressParams {
	t.Helper()
	var p protocol.ProgressParams
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("ProgressParams unmarshal: %v", err)
	}
	return p
}
