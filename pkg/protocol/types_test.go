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
