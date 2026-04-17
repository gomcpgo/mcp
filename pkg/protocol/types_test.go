package protocol

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		request  Request
		wantJSON string
	}{
		{
			name: "basic request",
			request: Request{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "test",
				Params:  json.RawMessage(`{"key":"value"}`),
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"method":"test","params":{"key":"value"}}`,
		},
		{
			name: "request without params",
			request: Request{
				JSONRPC: "2.0",
				ID:      "abc",
				Method:  "test",
			},
			wantJSON: `{"jsonrpc":"2.0","id":"abc","method":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.request)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			// Create normalized versions for comparison
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				t.Errorf("failed to unmarshal got json: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantJSON); err != nil {
				t.Errorf("failed to unmarshal want json: %v", err)
				return
			}

			gotStr, err := json.Marshal(gotJSON)
			if err != nil {
				t.Errorf("failed to marshal got json: %v", err)
				return
			}
			wantStr, err := json.Marshal(wantJSON)
			if err != nil {
				t.Errorf("failed to marshal want json: %v", err)
				return
			}

			if string(gotStr) != string(wantStr) {
				t.Errorf("json.Marshal() = %v, want %v", string(got), tt.wantJSON)
			}
		})
	}
}

func TestResponseMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantJSON string
	}{
		{
			name: "success response",
			response: Response{
				JSONRPC: "2.0",
				ID:      1,
				Result:  map[string]interface{}{"data": "test"},
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"result":{"data":"test"}}`,
		},
		{
			name: "error response",
			response: Response{
				JSONRPC: "2.0",
				ID:      1,
				Error: &Error{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.response)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			// Create normalized versions for comparison
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				t.Errorf("failed to unmarshal got json: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantJSON); err != nil {
				t.Errorf("failed to unmarshal want json: %v", err)
				return
			}

			gotStr, err := json.Marshal(gotJSON)
			if err != nil {
				t.Errorf("failed to marshal got json: %v", err)
				return
			}
			wantStr, err := json.Marshal(wantJSON)
			if err != nil {
				t.Errorf("failed to marshal want json: %v", err)
				return
			}

			if string(gotStr) != string(wantStr) {
				t.Errorf("json.Marshal() = %v, want %v", string(got), tt.wantJSON)
			}
		})
	}
}

func TestNotificationMarshaling(t *testing.T) {
	n := Notification{
		JSONRPC: "2.0",
		Method:  "notifications/tools/list_changed",
		Params:  json.RawMessage(`{"foo":"bar"}`),
	}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, hasID := parsed["id"]; hasID {
		t.Error("Notification must not include an id field")
	}
	if parsed["method"] != "notifications/tools/list_changed" {
		t.Errorf("method = %v, want notifications/tools/list_changed", parsed["method"])
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", parsed["jsonrpc"])
	}
}

func TestNewNotification(t *testing.T) {
	n, err := NewNotification("notifications/progress", map[string]interface{}{
		"progressToken": "abc",
		"progress":      0.5,
	})
	if err != nil {
		t.Fatalf("NewNotification failed: %v", err)
	}
	if n.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", n.JSONRPC)
	}
	if n.Method != "notifications/progress" {
		t.Errorf("Method = %q, want notifications/progress", n.Method)
	}
	if len(n.Params) == 0 {
		t.Error("Params should be populated")
	}
}

func TestNewNotificationNilParams(t *testing.T) {
	n, err := NewNotification("notifications/initialized", nil)
	if err != nil {
		t.Fatalf("NewNotification with nil params failed: %v", err)
	}
	if n.Method != "notifications/initialized" {
		t.Errorf("Method = %q, want notifications/initialized", n.Method)
	}
}

func TestProtocolVersion(t *testing.T) {
	if Version != "2025-11-25" {
		t.Errorf("Version = %q, want %q", Version, "2025-11-25")
	}
}

func TestSupportedVersions(t *testing.T) {
	if len(SupportedVersions) == 0 {
		t.Fatal("SupportedVersions must not be empty")
	}

	hasLatest := false
	hasLegacy := false
	for _, v := range SupportedVersions {
		if v == "2025-11-25" {
			hasLatest = true
		}
		if v == "2024-11-05" {
			hasLegacy = true
		}
	}
	if !hasLatest {
		t.Error("SupportedVersions missing 2025-11-25")
	}
	if !hasLegacy {
		t.Error("SupportedVersions missing 2024-11-05 (backward compat)")
	}
}

func TestInitializeRequestUnmarshaling(t *testing.T) {
	input := `{
		"protocolVersion": "2025-11-25",
		"clientInfo": {"name": "test-client", "version": "1.2.3"},
		"capabilities": {}
	}`

	var req InitializeRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if req.ProtocolVersion != "2025-11-25" {
		t.Errorf("ProtocolVersion = %q, want %q", req.ProtocolVersion, "2025-11-25")
	}
	if req.ClientInfo.Name != "test-client" {
		t.Errorf("ClientInfo.Name = %q, want %q", req.ClientInfo.Name, "test-client")
	}
	if req.ClientInfo.Version != "1.2.3" {
		t.Errorf("ClientInfo.Version = %q, want %q", req.ClientInfo.Version, "1.2.3")
	}
}

func TestCapabilitiesListChanged(t *testing.T) {
	caps := Capabilities{
		Tools:     &ToolsInfo{ListChanged: true},
		Resources: &ResourcesInfo{ListChanged: true, Subscribe: true},
		Prompts:   &PromptsInfo{ListChanged: true},
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	tools, ok := parsed["tools"].(map[string]interface{})
	if !ok {
		t.Fatal("tools missing from capabilities")
	}
	if tools["listChanged"] != true {
		t.Errorf("tools.listChanged = %v, want true", tools["listChanged"])
	}

	resources, ok := parsed["resources"].(map[string]interface{})
	if !ok {
		t.Fatal("resources missing from capabilities")
	}
	if resources["listChanged"] != true {
		t.Errorf("resources.listChanged = %v, want true", resources["listChanged"])
	}
	if resources["subscribe"] != true {
		t.Errorf("resources.subscribe = %v, want true", resources["subscribe"])
	}

	prompts, ok := parsed["prompts"].(map[string]interface{})
	if !ok {
		t.Fatal("prompts missing from capabilities")
	}
	if prompts["listChanged"] != true {
		t.Errorf("prompts.listChanged = %v, want true", prompts["listChanged"])
	}
}

func TestToolMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		tool     Tool
		wantJSON string
	}{
		{
			name: "basic tool",
			tool: Tool{
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"test":{"type":"string"}}}`),
			},
			wantJSON: `{
				"name": "test-tool",
				"description": "A test tool",
				"inputSchema": {"type":"object","properties":{"test":{"type":"string"}}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.tool)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			// Create normalized versions for comparison
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				t.Errorf("failed to unmarshal got json: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantJSON); err != nil {
				t.Errorf("failed to unmarshal want json: %v", err)
				return
			}

			gotStr, err := json.Marshal(gotJSON)
			if err != nil {
				t.Errorf("failed to marshal got json: %v", err)
				return
			}
			wantStr, err := json.Marshal(wantJSON)
			if err != nil {
				t.Errorf("failed to marshal want json: %v", err)
				return
			}

			if string(gotStr) != string(wantStr) {
				t.Errorf("json.Marshal() = %v, want %v", string(got), tt.wantJSON)
			}
		})
	}
}
